package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kuroki0x/rclob/internal/config"
	"github.com/kuroki0x/rclob/internal/handler"
	"github.com/kuroki0x/rclob/internal/logger"
	"github.com/kuroki0x/rclob/internal/repository"
	"github.com/kuroki0x/rclob/internal/service"
	"github.com/kuroki0x/rclob/pkg/redis"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// Initialize logger
	log, err := logger.New(cfg.LogLevel, cfg.LogFormat)
	if err != nil {
		panic(err)
	}
	defer logger.Sync(log)

	log.Info("starting rclob service",
		zap.String("port", cfg.Port),
		zap.String("redis_addr", cfg.RedisAddr),
	)

	// Initialize Redis client
	rdb, err := redis.NewClient(redis.Config{
		Addr:       cfg.RedisAddr,
		Password:   cfg.RedisPassword,
		DB:         cfg.RedisDB,
		MaxRetries: cfg.RedisMaxRetries,
	})
	if err != nil {
		log.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer rdb.Close()

	log.Info("connected to Redis")

	// Initialize layers
	repo := repository.NewOrderBookRepository(rdb.Client())
	svc := service.NewOrderBookService(repo)
	orderHandler := handler.NewOrderHandler(svc, log)

	// Setup router
	r := setupRouter(log, orderHandler)

	// Create HTTP server
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Start server
	go func() {
		log.Info("listening", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("server forced to shutdown", zap.Error(err))
	}

	log.Info("server stopped")
}

// setupRouter creates and configures the chi router
func setupRouter(log *zap.Logger, orderHandler *handler.OrderHandler) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Order routes
	orderHandler.RegisterRoutes(r)

	return r
}
