// filepath: /Users/uchebnick/projects/tg-serverless-api/api/cmd/api/main.go
package main

import (
	"context"
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"serverless-api/api/config"
	handlerpkg "serverless-api/api/internal/handler"
	botsrepo "serverless-api/api/internal/repository/bots"
	shardrepo "serverless-api/api/internal/repository/sharding"
)

func main() {
	cfg := config.Load()

	logger, err := zap.NewProduction()
	if err != nil {
		logger = zap.NewNop()
	}
	defer func() { _ = logger.Sync() }()

	sugar := logger.Sugar()
	sugar.Info("starting api server")

	listen := ":" + cfg.HTTPPort

	dbURL := cfg.DatabaseURL
	rdbAddr := cfg.SystemRedisAddr

	// init postgres
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		sugar.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	db.SetConnMaxLifetime(time.Minute * 5)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		sugar.Warnf("warning: failed to ping db: %v", err)
	}

	botsRepo := botsrepo.NewPostgresRepository(db)

	// init redis sharding repo
	rdb := redis.NewClient(&redis.Options{Addr: rdbAddr, Password: cfg.SystemRedisPass})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		sugar.Warnf("warning: failed to ping redis: %v", err)
	}
	shardingRepo := shardrepo.NewRedisRepository(rdb, 1)

	// init handlers
	h := &handlerpkg.Handlers{
		BotsRepo:     botsRepo,
		ShardingRepo: shardingRepo,
		K8S:          nil, // not configured here; see notes below how to enable via kubeconfig
		Logger:       logger,
	}

	app := fiber.New()
	h.RegisterRoutes(app)

	sugar.Infof("listening on %s", listen)
	if err := app.Listen(listen); err != nil {
		sugar.Fatalf("server failed: %v", err)
	}
}
