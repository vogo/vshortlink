/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package redisx

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// statsKeyFormat: one HASH per day, field=code, value=count.
	// Keeping the partition by day lets us expire whole days at once via TTL.
	statsKeyFormat = "shortlink:stats:%s" // shortlink:stats:{day}
)

// RedisShortLinkStats implements cores.ShortLinkStats using one Redis HASH
// per day. Retention is enforced by setting EXPIRE on the day-hash at write
// time, so no cleanup job is needed.
type RedisShortLinkStats struct {
	redis         *redis.Client
	keyPrefix     string
	retentionDays int
}

type StatsOption func(*RedisShortLinkStats)

func WithStatsKeyPrefix(prefix string) StatsOption {
	return func(s *RedisShortLinkStats) {
		s.keyPrefix = prefix
	}
}

// WithStatsRetentionDays controls the TTL applied to each day-hash on write.
// Must match (or exceed) the service-level retention; defaults to 7.
func WithStatsRetentionDays(days int) StatsOption {
	return func(s *RedisShortLinkStats) {
		if days > 0 {
			s.retentionDays = days
		}
	}
}

func NewRedisShortLinkStats(redisClient *redis.Client, opts ...StatsOption) *RedisShortLinkStats {
	s := &RedisShortLinkStats{
		redis:         redisClient,
		retentionDays: 7,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *RedisShortLinkStats) getStatsKey(day string) string {
	return s.keyPrefix + fmt.Sprintf(statsKeyFormat, day)
}

// ttl includes one extra day as a buffer so queries on the oldest-retained day
// still find the key if the caller's clock trails the retention window.
func (s *RedisShortLinkStats) ttl() time.Duration {
	return time.Duration(s.retentionDays+1) * 24 * time.Hour
}

// IncrBatch pipelines HINCRBY for every code and re-applies EXPIRE on the
// day-hash. Re-applying EXPIRE every flush is idempotent with the same value
// and keeps the TTL anchored to the last write rather than the first one.
func (s *RedisShortLinkStats) IncrBatch(ctx context.Context, day string, deltas map[string]int64) error {
	if len(deltas) == 0 {
		return nil
	}
	key := s.getStatsKey(day)

	pipe := s.redis.Pipeline()
	for code, delta := range deltas {
		pipe.HIncrBy(ctx, key, code, delta)
	}
	pipe.Expire(ctx, key, s.ttl())
	_, err := pipe.Exec(ctx)
	return err
}

// Get pipelines one HGET per day. days is expected to be small (== retentionDays).
func (s *RedisShortLinkStats) Get(ctx context.Context, code string, days []string) (map[string]int64, error) {
	if len(days) == 0 {
		return map[string]int64{}, nil
	}

	pipe := s.redis.Pipeline()
	cmds := make([]*redis.StringCmd, len(days))
	for i, day := range days {
		cmds[i] = pipe.HGet(ctx, s.getStatsKey(day), code)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	out := make(map[string]int64, len(days))
	for i, cmd := range cmds {
		v, err := cmd.Int64()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		if v != 0 {
			out[days[i]] = v
		}
	}
	return out, nil
}

// BatchGet pipelines one HMGET per day. Total commands == len(days),
// independent of len(codes) — this is the reason downstream jobs should prefer
// batches over per-code loops.
func (s *RedisShortLinkStats) BatchGet(ctx context.Context, codes []string, days []string) (map[string]map[string]int64, error) {
	out := make(map[string]map[string]int64, len(codes))
	if len(codes) == 0 || len(days) == 0 {
		return out, nil
	}

	pipe := s.redis.Pipeline()
	cmds := make([]*redis.SliceCmd, len(days))
	for i, day := range days {
		cmds[i] = pipe.HMGet(ctx, s.getStatsKey(day), codes...)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	for i, cmd := range cmds {
		values, err := cmd.Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		day := days[i]
		for j, raw := range values {
			if raw == nil {
				continue
			}
			str, ok := raw.(string)
			if !ok {
				continue
			}
			n, err := strconv.ParseInt(str, 10, 64)
			if err != nil || n == 0 {
				continue
			}
			code := codes[j]
			dayMap, exists := out[code]
			if !exists {
				dayMap = make(map[string]int64)
				out[code] = dayMap
			}
			dayMap[day] = n
		}
	}
	return out, nil
}

// Close is a no-op; the redis.Client is owned by the caller.
func (s *RedisShortLinkStats) Close(ctx context.Context) error {
	return nil
}
