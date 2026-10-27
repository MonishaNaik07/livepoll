package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-username/live-polling-app/backend/internal/apperr"
)

type Base struct{ Log *slog.Logger }

func ok(c *gin.Context, status int, data interface{}) { c.JSON(status, gin.H{"data": data}) }

// fail is the single place errors become HTTP responses. Unknown errors are logged and masked.
func (b *Base) fail(c *gin.Context, err error) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		c.JSON(ae.Status, gin.H{"error": ae})
		return
	}
	b.Log.Error("unhandled error", "route", c.FullPath(), "err", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": apperr.ErrInternal})
}

func (b *Base) bind(c *gin.Context, dst interface{}) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			b.fail(c, apperr.New(http.StatusRequestEntityTooLarge, "payload_too_large", "Request body too large"))
		} else {
			b.fail(c, apperr.New(http.StatusBadRequest, "invalid_json", "Malformed JSON body"))
		}
		return false
	}
	return true
}

type Cookies struct {
	Secure   bool
	SameSite http.SameSite
}

func ParseSameSite(s string) http.SameSite {
	switch s {
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	}
	return http.SameSiteLaxMode
}

func (k Cookies) set(c *gin.Context, name, val string, ttl time.Duration) {
	c.SetSameSite(k.SameSite)
	c.SetCookie(name, val, int(ttl.Seconds()), "/", "", k.Secure, true) // HttpOnly always
}

func (k Cookies) clear(c *gin.Context, name string) {
	c.SetSameSite(k.SameSite)
	c.SetCookie(name, "", -1, "/", "", k.Secure, true)
}

const voterCookie = "lp_voter"

// VoterID returns the anonymous voter identity, minting a random 128-bit id cookie if absent.
func (k Cookies) VoterID(c *gin.Context) string {
	if v, err := c.Cookie(voterCookie); err == nil && len(v) == 32 {
		if _, derr := hex.DecodeString(v); derr == nil {
			return v
		}
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	k.set(c, voterCookie, id, 365*24*time.Hour)
	return id
}
