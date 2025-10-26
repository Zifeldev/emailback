package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Zifeldev/emailback/internal/config"
	"github.com/Zifeldev/emailback/internal/controllers"
	"github.com/Zifeldev/emailback/internal/db"
	"github.com/Zifeldev/emailback/internal/lang"
	"github.com/Zifeldev/emailback/internal/middleware"
	"github.com/Zifeldev/emailback/internal/repository"
	"github.com/Zifeldev/emailback/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/pemistahl/lingua-go"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.MustLoad(context.Background())

	log := logrus.New()
	if lvl, err := logrus.ParseLevel(cfg.Logger.Level); err == nil {
		log.SetLevel(lvl)
	}

	pool, err := db.New(context.Background(), cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("failed to connect to database")
	}
	defer pool.Close()

	ld := lang.NewDetector(lingua.English, lingua.Russian, lingua.German)

	emailRepo := repository.NewPostgresEmailRepo(pool)

	emailParser := service.NewEnmimeParser(service.Options{
		HTMLToTextLimit: 1 << 20,
		IncludeHTML:     false,
	}, ld)

	r := gin.New()

	r.Use(middleware.RecoveryMiddleware(log))
	r.Use(middleware.TraceMiddleware(log))
	r.Use(middleware.LoggerMiddleware(log))

	r.GET("/health", handler.HealthHandler)
	r.GET("/healthz", handler.NewDBHealth(pool).Handle)
	r.POST("/parse", handler.NewParserController(emailParser,emailRepo,log).ParseAndSave)
	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"message": "Not Found"})
	})

	

	srv := &http.Server{
		Addr:              cfg.HTTP.Host,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	srv.RegisterOnShutdown(func() {
		log.Info("closing database connection pool")
		pool.Close()
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.WithField("addr", cfg.HTTP.Host).Info("starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("http server failed")
		}
	}()
	<-ctx.Done()
	log.Info("shutting down gracefully, press Ctrl+C again to force")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Fatal("http server shutdown failed")
	} else {
		log.Info("server exited properly")
	}
}
