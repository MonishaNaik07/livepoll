package routes

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-username/live-polling-app/backend/internal/config"
	"github.com/your-username/live-polling-app/backend/internal/handlers"
	"github.com/your-username/live-polling-app/backend/internal/middleware"
	rds "github.com/your-username/live-polling-app/backend/internal/redis"
	"github.com/your-username/live-polling-app/backend/internal/services"
	ws "github.com/your-username/live-polling-app/backend/internal/websocket"
	"go.mongodb.org/mongo-driver/mongo"
)

type Deps struct {
	Cfg   *config.Config
	Log   *slog.Logger
	Mongo *mongo.Client
	Store *rds.Store
	Auth  *services.AuthService
	Polls *services.PollService
	Votes *services.VoteService
	Hub   *ws.Hub
}

func New(d Deps) *gin.Engine {
	if d.Cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.Recovery(d.Log), middleware.Logger(d.Log), middleware.CORS(d.Cfg.FrontendOrigin),
		middleware.BodyLimit(d.Cfg.MaxBodyBytes))

	// System
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		checks := gin.H{"mongo": "ok", "redis": "ok"}
		code := 200
		if err := d.Mongo.Ping(ctx, nil); err != nil {
			checks["mongo"], code = "unavailable", 503
		}
		if err := d.Store.Ping(ctx); err != nil {
			checks["redis"], code = "unavailable", 503
		}
		c.JSON(code, gin.H{"status": map[bool]string{true: "ready", false: "not_ready"}[code == 200], "checks": checks})
	})

	base := handlers.Base{Log: d.Log}
	ck := handlers.Cookies{Secure: d.Cfg.CookieSecure, SameSite: handlers.ParseSameSite(d.Cfg.CookieSameSite)}
	ah := handlers.NewAuthHandler(base, d.Auth, ck)
	ph := handlers.NewPollHandler(base, d.Polls, d.Votes, ck, d.Cfg.PublicAppURL)

	rl := func(scope string, n int64, w time.Duration) gin.HandlerFunc {
		return middleware.RateLimit(d.Store.C, scope, n, w, d.Log)
	}
	required := middleware.Auth(d.Auth, true)
	optional := middleware.Auth(d.Auth, false)

	api := r.Group("/api", middleware.CSRF(d.Cfg.FrontendOrigin))
	{
		a := api.Group("/auth")
		a.POST("/signup", rl("signup", 10, time.Hour), ah.Signup)
		a.POST("/login", rl("login", 10, 5*time.Minute), ah.Login)
		a.POST("/logout", ah.Logout)
		a.GET("/me", required, ah.Me)

		api.POST("/polls", required, ph.Create)
		api.GET("/polls", required, ph.List)
		api.GET("/polls/:id", optional, ph.Get)
		api.GET("/polls/:id/results", ph.Results)
		api.GET("/polls/:id/share", ph.Share)
		api.GET("/polls/:id/export", required, ph.ExportCSV)
		api.POST("/polls/:id/close", required, ph.Close)
		api.POST("/polls/:id/reopen", required, ph.Reopen)
		api.DELETE("/polls/:id", required, ph.Delete)
		api.POST("/polls/:id/votes", rl("vote", 30, time.Minute), ph.Vote)

		api.GET("/ws/polls/:id", rl("ws", 60, time.Minute), d.Hub.Serve)
	}

	// Optional same-origin hosting of the built SPA (avoids third-party-cookie problems in Safari).
	if dir := d.Cfg.StaticDir; dir != "" {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(404, gin.H{"error": gin.H{"code": "not_found", "message": "Route not found"}})
				return
			}
			p := filepath.Join(dir, filepath.Clean("/"+c.Request.URL.Path))
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				c.File(p)
				return
			}
			c.File(filepath.Join(dir, "index.html"))
		})
	} else {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "Route not found"}})
		})
	}
	return r
}
