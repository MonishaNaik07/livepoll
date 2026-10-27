package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-username/live-polling-app/backend/internal/config"
	"github.com/your-username/live-polling-app/backend/internal/database"
	rds "github.com/your-username/live-polling-app/backend/internal/redis"
	"github.com/your-username/live-polling-app/backend/internal/repositories"
	"github.com/your-username/live-polling-app/backend/internal/routes"
	"github.com/your-username/live-polling-app/backend/internal/services"
	ws "github.com/your-username/live-polling-app/backend/internal/websocket"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mongoClient, db, err := database.ConnectMongo(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Error("mongo", "err", err)
		os.Exit(1)
	}
	store := rds.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisTLS)
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := store.Ping(pctx); err != nil {
		cancel()
		log.Error("redis", "err", err)
		os.Exit(1)
	}
	cancel()

	userRepo, pollRepo, voteRepo := repositories.NewUserRepo(db), repositories.NewPollRepo(db), repositories.NewVoteRepo(db)
	auth := services.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTTTL)
	polls := services.NewPollService(pollRepo, voteRepo, store, log)
	votes := services.NewVoteService(polls, voteRepo, store, log)
	hub := ws.NewHub(store, polls, log, cfg.FrontendOrigin)

	go hub.Run(ctx)
	go votes.RunReconciler(ctx, 15*time.Second)

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: routes.New(routes.Deps{Cfg: cfg, Log: log, Mongo: mongoClient, Store: store,
			Auth: auth, Polls: polls, Votes: votes, Hub: hub}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Info("listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	sctx, scancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer scancel()
	hub.Shutdown()
	if err := srv.Shutdown(sctx); err != nil {
		log.Error("shutdown", "err", err)
	}
	_ = mongoClient.Disconnect(sctx)
	_ = store.Close()
}
