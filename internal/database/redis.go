package database

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/xboard/xboard/internal/config"
)

var Redis *redis.Client

func InitRedis(cfg *config.RedisConfig) *redis.Client {
	return InitRedisWithTimeout(cfg)
}

func InitRedisWithTimeout(cfg *config.RedisConfig) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     20,
		MinIdleConns: 5,
		MaxRetries:   3,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("redis ping failed (non-fatal): %v", err)
	}

	Redis = rdb
	return rdb
}
