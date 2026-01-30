package dedupe

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"realtime-message/internal/config"
	"realtime-message/internal/model"
)

type Store struct {
	client    *redis.Client
	prefix    string
	keyStrategy []string
	ttl        time.Duration
}

func New(cfg config.RedisConfig, dcfg config.DedupeConfig) *Store {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ttl := time.Duration(dcfg.TTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 72 * time.Hour
	}
	return &Store{client: client, prefix: cfg.KeyPrefix, keyStrategy: dcfg.KeyStrategy, ttl: ttl}
}

func (s *Store) Seen(ctx context.Context, msg model.Message) (bool, string, error) {
	keys := buildKeys(s.keyStrategy, msg)
	for _, k := range keys {
		full := s.prefix + k
		ok, err := s.client.SetNX(ctx, full, 1, s.ttl).Result()
		if err != nil {
			return false, k, err
		}
		if ok {
			return false, k, nil
		}
		return true, k, nil
	}
	return false, "", nil
}

func buildKeys(strategy []string, msg model.Message) []string {
	if len(strategy) == 0 {
		strategy = []string{"url", "id", "source_title", "source_title_time"}
	}
	keys := []string{}
	for _, s := range strategy {
		switch s {
		case "url":
			if msg.URL != "" {
				keys = append(keys, fmt.Sprintf("url:%s", msg.URL))
			}
		case "id":
			if msg.ID != "" {
				keys = append(keys, fmt.Sprintf("id:%s", msg.ID))
			}
		case "source_title":
			if msg.Title != "" {
				keys = append(keys, fmt.Sprintf("st:%s:%s", msg.Source, msg.Title))
			}
		case "source_title_time":
			if msg.Title != "" {
				keys = append(keys, fmt.Sprintf("stt:%s:%s:%s", msg.Source, msg.Title, msg.Time.Format(time.RFC3339)))
			}
		}
	}
	return keys
}
