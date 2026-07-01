package health

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultTTL = 30 * time.Second

type Store struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewStore(rdb *redis.Client, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Store{rdb: rdb, ttl: ttl}
}

func (s *Store) SetInstanceHealth(ctx context.Context, ih InstanceHealth) error {
	if ih.Status == "" {
		ih.Status = StatusUnknown
	}
	if ih.LastCheck.IsZero() {
		ih.LastCheck = time.Now().UTC()
	}

	key := instanceKey(ih.InstanceID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"instance_id": ih.InstanceID,
		"service_id":  ih.ServiceID,
		"status":      string(ih.Status),
		"last_check":  ih.LastCheck.Format(time.RFC3339Nano),
		"latency_ms":  strconv.FormatFloat(ih.LatencyMS, 'f', -1, 64),
		"fail_count":  strconv.Itoa(ih.FailCount),
	})
	pipe.Expire(ctx, key, s.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) GetInstanceHealth(ctx context.Context, instanceID string) (InstanceHealth, error) {
	values, err := s.rdb.HGetAll(ctx, instanceKey(instanceID)).Result()
	if err != nil {
		return InstanceHealth{}, err
	}
	if len(values) == 0 {
		return InstanceHealth{InstanceID: instanceID, Status: StatusUnknown}, nil
	}

	ih := InstanceHealth{
		InstanceID: valueOr(values, "instance_id", instanceID),
		ServiceID:  values["service_id"],
		Status:     Status(valueOr(values, "status", string(StatusUnknown))),
	}
	if ih.Status == "" {
		ih.Status = StatusUnknown
	}
	if raw := values["latency_ms"]; raw != "" {
		ih.LatencyMS, _ = strconv.ParseFloat(raw, 64)
	}
	if raw := values["fail_count"]; raw != "" {
		ih.FailCount, _ = strconv.Atoi(raw)
	}
	if raw := values["last_check"]; raw != "" {
		ih.LastCheck, _ = time.Parse(time.RFC3339Nano, raw)
	}

	return ih, nil
}

func (s *Store) MarkAlive(ctx context.Context, serviceID, instanceID string) error {
	return s.rdb.SAdd(ctx, aliveKey(serviceID), instanceID).Err()
}

func (s *Store) MarkDown(ctx context.Context, serviceID, instanceID string) error {
	return s.rdb.SRem(ctx, aliveKey(serviceID), instanceID).Err()
}

func (s *Store) AliveSet(ctx context.Context, serviceID string) (map[string]struct{}, error) {
	members, err := s.rdb.SMembers(ctx, aliveKey(serviceID)).Result()
	if errors.Is(err, redis.Nil) {
		return map[string]struct{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(members))
	for _, member := range members {
		out[member] = struct{}{}
	}
	return out, nil
}

func instanceKey(instanceID string) string {
	return fmt.Sprintf("health:instance:%s", instanceID)
}

func aliveKey(serviceID string) string {
	return fmt.Sprintf("health:service:%s:alive", serviceID)
}

func valueOr(values map[string]string, key string, fallback string) string {
	if values[key] == "" {
		return fallback
	}
	return values[key]
}
