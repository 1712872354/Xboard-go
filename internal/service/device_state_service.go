package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/xboard/xboard/internal/database"
	"gorm.io/gorm"
)

// DeviceStateService manages user online device tracking
type DeviceStateService struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewDeviceStateService(db *gorm.DB) *DeviceStateService {
	return &DeviceStateService{
		db:  db,
		rdb: database.Redis,
	}
}

const (
	deviceKeyPrefix = "user_devices:"
	deviceTTL       = 300 * time.Second // 5 minutes
)

// SetDevices records user's online devices from a specific node
func (s *DeviceStateService) SetDevices(userID uint, nodeID uint, ips []string) error {
	if s.rdb == nil {
		return nil // no Redis, skip
	}

	ctx := context.Background()
	key := fmt.Sprintf("%s%d", deviceKeyPrefix, userID)

	// Get existing devices
	existing, _ := s.rdb.HGetAll(ctx, key).Result()

	// Merge new devices
	devices := existing
	for _, ip := range ips {
		deviceKey := fmt.Sprintf("%d:%s", nodeID, ip)
		devices[deviceKey] = fmt.Sprintf("%d", time.Now().Unix())
	}

	// Store merged devices
	if len(devices) > 0 {
		s.rdb.HSet(ctx, key, devices)
		s.rdb.Expire(ctx, key, deviceTTL)
	}

	return nil
}

// GetDevices returns all online devices for a user
func (s *DeviceStateService) GetDevices(userID uint) ([]DeviceInfo, error) {
	if s.rdb == nil {
		return nil, nil
	}

	ctx := context.Background()
	key := fmt.Sprintf("%s%d", deviceKeyPrefix, userID)

	devices, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var result []DeviceInfo
	for deviceKey, ts := range devices {
		var nodeID uint
		var ip string
		fmt.Sscanf(deviceKey, "%d:%s", &nodeID, &ip)
		var timestamp int64
		fmt.Sscanf(ts, "%d", &timestamp)

		result = append(result, DeviceInfo{
			NodeID:    nodeID,
			IP:        ip,
			LastSeen:  time.Unix(timestamp, 0),
		})
	}

	return result, nil
}

// RemoveDevice removes a specific device from online devices
func (s *DeviceStateService) RemoveDevice(userID uint, nodeID uint, ip string) error {
	if s.rdb == nil {
		return nil
	}

	ctx := context.Background()
	key := fmt.Sprintf("%s%d", deviceKeyPrefix, userID)
	deviceKey := fmt.Sprintf("%d:%s", nodeID, ip)

	return s.rdb.HDel(ctx, key, deviceKey).Err()
}

// ClearDevices clears all online devices for a user
func (s *DeviceStateService) ClearDevices(userID uint) error {
	if s.rdb == nil {
		return nil
	}

	ctx := context.Background()
	key := fmt.Sprintf("%s%d", deviceKeyPrefix, userID)
	return s.rdb.Del(ctx, key).Err()
}

// CheckDeviceLimit checks if user has exceeded device limit
func (s *DeviceStateService) CheckDeviceLimit(userID uint, limit int) (bool, error) {
	if s.rdb == nil || limit <= 0 {
		return true, nil // no limit or no Redis
	}

	devices, err := s.GetDevices(userID)
	if err != nil {
		return true, err
	}

	// Count unique IPs (same IP on different nodes counts as one device)
	ipSet := make(map[string]bool)
	for _, d := range devices {
		ipSet[d.IP] = true
	}

	return len(ipSet) < limit, nil
}

// DeviceInfo represents an online device
type DeviceInfo struct {
	NodeID   uint      `json:"node_id"`
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
}
