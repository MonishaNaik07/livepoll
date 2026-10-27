package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/your-username/live-polling-app/backend/internal/apperr"
	"github.com/your-username/live-polling-app/backend/internal/services"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	AuthCookie = "lp_token"
	userKey    = "uid"
)

func abort(c *gin.Context, e *apperr.Error) {
	c.AbortWithStatusJSON(e.Status, gin.H{"error": e})
}

// CORS allows exactly one origin (the frontend) with credentials.
func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if o := c.GetHeader("Origin"); o != "" && o == origin {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Content-Type, X-Requested-With")
			h.Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
			h.Set("Access-Control-Max-Age", "600")
			h.Add("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// CSRF protects cookie-authenticated state-changing requests: a custom header (forces a CORS
// preflight that only the allowed origin passes) plus an Origin check.
func CSRF(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if o := c.GetHeader("Origin"); o != "" && o != origin {
			abort(c, apperr.New(http.StatusForbidden, "bad_origin", "Origin not allowed"))
			return
		}
		if c.GetHeader("X-Requested-With") != "livepoll" {
			abort(c, apperr.New(http.StatusForbidden, "csrf", "Missing CSRF header"))
			return
		}
		c.Next()
	}
}

func BodyLimit(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

func Logger(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		// FullPath() is the route pattern: no query strings, tokens or raw IDs end up in logs.
		l.Info("request", "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(),
			"ms", time.Since(start).Milliseconds(), "ip", c.ClientIP())
	}
}

func Recovery(l *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		l.Error("panic recovered", "route", c.FullPath(), "panic", fmt.Sprint(recovered))
		abort(c, apperr.ErrInternal)
	})
}

// Auth validates the JWT cookie. required=false lets anonymous visitors through (owner-aware public pages).
func Auth(a *services.AuthService, required bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		tok, err := c.Cookie(AuthCookie)
		if err == nil && tok != "" {
			if id, perr := a.Parse(tok); perr == nil {
				c.Set(userKey, id)
				c.Next()
				return
			}
		}
		if required {
			abort(c, apperr.ErrUnauthorized)
			return
		}
		c.Next()
	}
}

func UserID(c *gin.Context) (primitive.ObjectID, bool) {
	v, ok := c.Get(userKey)
	if !ok {
		return primitive.NilObjectID, false
	}
	id, ok := v.(primitive.ObjectID)
	return id, ok
}

// RateLimit is a Redis fixed-window limiter (shared across instances). Fails open if Redis is down
// so an outage doesn't block logins; the failure is logged.
func RateLimit(r *redis.Client, scope string, limit int64, window time.Duration, l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rl:%s:%s", scope, c.ClientIP())
		ctx := c.Request.Context()
		pipe := r.TxPipeline()
		incr := pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, window)
		if _, err := pipe.Exec(ctx); err != nil {
			l.Warn("rate limiter unavailable", "scope", scope, "err", err)
			c.Next()
			return
		}
		if incr.Val() > limit {
			c.Header("Retry-After", strconv.Itoa(int(window.Seconds())))
			abort(c, apperr.ErrRateLimited)
			return
		}
		c.Next()
	}
}
