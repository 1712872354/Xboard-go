package v1

import (
	"github.com/redis/go-redis/v9"
	"github.com/xboard/xboard/internal/database"
)

// TrafficRecordV1 represents a single user's traffic record from node
type TrafficRecordV1 struct {
	UserID uint  `json:"user_id"`
	U      int64 `json:"u"`
	D      int64 `json:"d"`
}

// getRedisClient returns the global Redis client if configured
func getRedisClient() *redis.Client {
	return database.Redis
}
