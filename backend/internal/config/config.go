package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	Env            string
	FrontendOrigin string
	PublicAppURL   string
	MongoURI       string
	MongoDB        string
	RedisAddr      string
	RedisPassword  string
	RedisTLS       bool
	JWTSecret      string
	JWTTTL         time.Duration
	CookieSecure   bool
	CookieSameSite string
	MaxBodyBytes   int64
	StaticDir      string // optional: serve built frontend from the Go service (same-origin deploy)
}

func Load() (*Config, error) {
	c := &Config{
		Port:           get("PORT", "8080"),
		Env:            get("APP_ENV", "development"),
		FrontendOrigin: get("FRONTEND_ORIGIN", "http://localhost:5173"),
		PublicAppURL:   get("PUBLIC_APP_URL", "http://localhost:5173"),
		MongoURI:       os.Getenv("MONGO_URI"),
		MongoDB:        get("MONGO_DB", "livepoll"),
		RedisAddr:      get("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		RedisTLS:       getBool("REDIS_TLS", false),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		CookieSecure:   getBool("COOKIE_SECURE", false),
		CookieSameSite: strings.ToLower(get("COOKIE_SAMESITE", "lax")),
		MaxBodyBytes:   int64(getInt("MAX_BODY_BYTES", 65536)),
		StaticDir:      os.Getenv("STATIC_DIR"),
		JWTTTL:         time.Duration(getInt("JWT_TTL_HOURS", 24)) * time.Hour,
	}
	if c.MongoURI == "" {
		return nil, fmt.Errorf("MONGO_URI is required")
	}
	if len(c.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if c.Env == "production" && !c.CookieSecure {
		return nil, fmt.Errorf("COOKIE_SECURE must be true in production")
	}
	return c, nil
}

func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func getBool(k string, d bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return d
}

func getInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}
