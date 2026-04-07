package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	maxNotifications = 20
	notificationTTL  = 7 * 24 * time.Hour
)

// StoredNotification is a persistent notification entry.
type StoredNotification struct {
	ID        string `json:"id"`         // dedup key: "{type}:{auction_id}"
	Type      string `json:"type"`       // outbid, won, auction_closed
	AuctionID string `json:"auction_id"`
	ItemTitle string `json:"item_title"`
	Message   string `json:"message"`
	Link      string `json:"link"`
	Amount    int64  `json:"amount"`     // cents
	CreatedAt int64  `json:"created_at"` // Unix ms
	Read      bool   `json:"read"`
}

// UserNotificationMessage is sent over the user-level WebSocket.
type UserNotificationMessage struct {
	Type         string             `json:"type"`          // always "notification"
	Notification StoredNotification `json:"notification"`
	UnreadCount  int                `json:"unread_count"`
}

// Store handles Redis persistence for user notifications.
type Store struct {
	rdb *redis.Client
}

// NewStore creates a new notification Store.
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func notifKey(userID string) string       { return "notifications:" + userID }
func dedupKey(userID string) string       { return "notifications:" + userID + ":dedup" }
func unreadKey(userID string) string      { return "notifications:" + userID + ":unread" }

// Add stores a notification for a user, deduplicating by ID.
func (s *Store) Add(ctx context.Context, userID string, n StoredNotification) error {
	nKey := notifKey(userID)
	dKey := dedupKey(userID)
	data, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	pipe := s.rdb.Pipeline()

	// Remove old entry with same dedup key if it exists
	oldJSON, err := s.rdb.HGet(ctx, dKey, n.ID).Result()
	if err == nil && oldJSON != "" {
		pipe.ZRem(ctx, nKey, oldJSON)
	}

	// Add new entry
	pipe.ZAdd(ctx, nKey, redis.Z{
		Score:  float64(n.CreatedAt),
		Member: string(data),
	})
	pipe.HSet(ctx, dKey, n.ID, string(data))

	// Trim to max entries (remove oldest)
	pipe.ZRemRangeByRank(ctx, nKey, 0, int64(-maxNotifications-1))

	// Set TTL
	pipe.Expire(ctx, nKey, notificationTTL)
	pipe.Expire(ctx, dKey, notificationTTL)

	// Increment unread counter
	pipe.Incr(ctx, unreadKey(userID))
	pipe.Expire(ctx, unreadKey(userID), notificationTTL)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("store notification: %w", err)
	}
	return nil
}

// List returns the most recent notifications for a user (newest first).
func (s *Store) List(ctx context.Context, userID string) ([]StoredNotification, error) {
	vals, err := s.rdb.ZRevRange(ctx, notifKey(userID), 0, int64(maxNotifications-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	notifications := make([]StoredNotification, 0, len(vals))
	for _, v := range vals {
		var n StoredNotification
		if err := json.Unmarshal([]byte(v), &n); err != nil {
			continue
		}
		notifications = append(notifications, n)
	}
	return notifications, nil
}

// MarkAllRead sets all notifications as read and resets unread counter.
func (s *Store) MarkAllRead(ctx context.Context, userID string) error {
	nKey := notifKey(userID)
	dKey := dedupKey(userID)

	vals, err := s.rdb.ZRevRange(ctx, nKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("read notifications: %w", err)
	}

	if len(vals) == 0 {
		s.rdb.Set(ctx, unreadKey(userID), "0", notificationTTL)
		return nil
	}

	pipe := s.rdb.Pipeline()
	for _, v := range vals {
		var n StoredNotification
		if err := json.Unmarshal([]byte(v), &n); err != nil {
			continue
		}
		if n.Read {
			continue
		}
		// Remove old, add updated
		pipe.ZRem(ctx, nKey, v)
		n.Read = true
		data, _ := json.Marshal(n)
		pipe.ZAdd(ctx, nKey, redis.Z{Score: float64(n.CreatedAt), Member: string(data)})
		pipe.HSet(ctx, dKey, n.ID, string(data))
	}
	pipe.Set(ctx, unreadKey(userID), "0", notificationTTL)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}

// UnreadCount returns the current unread count for a user.
func (s *Store) UnreadCount(ctx context.Context, userID string) (int, error) {
	val, err := s.rdb.Get(ctx, unreadKey(userID)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get unread count: %w", err)
	}
	count, _ := strconv.Atoi(val)
	return count, nil
}
