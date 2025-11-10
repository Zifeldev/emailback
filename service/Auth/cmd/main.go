package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/Zifeldev/emailback/service/Auth/docs"
	"github.com/Zifeldev/emailback/service/Auth/internal/config"
	"github.com/Zifeldev/emailback/service/Auth/internal/controllers"
	"github.com/Zifeldev/emailback/service/Auth/internal/db"
	"github.com/Zifeldev/emailback/service/Auth/internal/logger"
	"github.com/Zifeldev/emailback/service/Auth/internal/middleware"
	"github.com/Zifeldev/emailback/service/Auth/internal/repository"
	"github.com/Zifeldev/emailback/service/Auth/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Auth Service API
// @version 1.0
// @description JWT-based authentication service with refresh tokens
// @host localhost:8081
// @BasePath /
func main() {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load(ctx)
	if err != nil {
		logrus.WithError(err).Fatal("failed to load config")
	}

	// Setup logger
	log := logger.New()
	baseEntry := log.WithField("service", "auth")

	baseEntry.WithFields(logrus.Fields{
		"http_addr":        cfg.HTTP.Host,
		"shutdown_timeout": cfg.HTTP.ShutdownTimeout,
		"req_timeout":      cfg.HTTP.RequestTimeout,
		"db_query_timeout": cfg.Database.QueryTimeout,
	}).Info("config loaded")

	// Connect to PostgreSQL
	pool, err := db.New(ctx, cfg.Database)
	if err != nil {
		baseEntry.WithError(err).Fatal("failed to connect to database")
	}
	defer pool.Close()

	// Connect to Redis
	var rdb *redis.Client
	if cfg.Redis.Enabled {
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})

		if err := rdb.Ping(ctx).Err(); err != nil {
			baseEntry.WithError(err).Fatal("failed to connect to redis")
		}
		defer rdb.Close()
		baseEntry.Info("redis connected")
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(pool, &cfg.JWT)
	tokenRepo := repository.NewTokenRepository(pool)

	// Initialize services
	authService := service.NewAuthService(&cfg.JWT, userRepo, tokenRepo)

	// Initialize controllers
	authController := controllers.NewAuthController(authService, baseEntry)
	adminController := controllers.NewAdminController(userRepo, baseEntry)
	healthController := controllers.NewHealthController(pool, rdb, baseEntry, time.Now(), "1.0.0")

	// Setup Gin
	if cfg.Logger.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Routes
	r.GET("/health", healthController.Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Auth routes (public)
	auth := r.Group("/auth")
	{
		auth.POST("/register", authController.Register)
		auth.POST("/login", authController.Login)
		auth.POST("/refresh", authController.Refresh)
		auth.POST("/logout", authController.Logout)
	}

	// Protected routes example
	protected := r.Group("/api")
	protected.Use(middleware.JWTAuth(authService))
	{
		protected.GET("/me", func(c *gin.Context) {
			userID, _ := middleware.GetUserID(c)
			email, _ := middleware.GetUserEmail(c)
			role, _ := middleware.GetUserRole(c)
			c.JSON(http.StatusOK, gin.H{
				"user_id": userID,
				"email":   email,
				"role":    role,
			})
		})
	}

	// Admin routes (admin only)
	admin := r.Group("/admin")
	admin.Use(middleware.JWTAuth(authService))
	admin.Use(middleware.RequireRole("admin"))
	{
		admin.GET("/users", adminController.ListUsers)
		admin.POST("/users", adminController.CreateUser)
		admin.PUT("/users/:id/role", adminController.UpdateUserRole)
		admin.DELETE("/users/:id", adminController.DeleteUser)
	}

	// Start server
	srv := &http.Server{
		Addr:    cfg.HTTP.Host,
		Handler: r,
	}

	go func() {
		baseEntry.WithField("addr", cfg.HTTP.Host).Info("starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			baseEntry.WithError(err).Fatal("server failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	baseEntry.WithField("signal", "terminated").WithField("grace_period_sec", cfg.HTTP.ShutdownTimeout).Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		baseEntry.WithError(err).Fatal("server forced to shutdown")
	}

	baseEntry.Info("server exited properly")
	baseEntry.Info("closing database connection pool")
}
