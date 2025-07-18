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
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes
	poolKeyFormat = "shortlink:pool:%d"      // shortlink:pool:{length}
	lockKeyFormat = "shortlink:pool:lock:%d" // shortlink:pool:lock:{length}
)

// ErrPoolLocked is returned when a short code pool is locked
var ErrPoolLocked = errors.New("short code pool is locked")

// RedisShortCodePool implements cores.ShortCodePool interface with Redis storage
type RedisShortCodePool struct {
	redis     *redis.Client
	keyPrefix string
}

type PoolOption func(c *RedisShortCodePool)

func WithPoolKeyPrefix(prefix string) PoolOption {
	return func(c *RedisShortCodePool) {
		c.keyPrefix = prefix
	}
}

// NewRedisShortCodePool creates a new RedisShortCodePool
func NewRedisShortCodePool(redisClient *redis.Client, opts ...PoolOption) *RedisShortCodePool {
	pool := &RedisShortCodePool{
		redis: redisClient,
	}

	for _, opt := range opts {
		opt(pool)
	}

	return pool
}

// getPoolKey returns the Redis key for the short code pool of the given length
func (p *RedisShortCodePool) getPoolKey(length int) string {
	return p.keyPrefix + fmt.Sprintf(poolKeyFormat, length)
}

// getLockKey returns the Redis key for the lock of the short code pool of the given length
func (p *RedisShortCodePool) getLockKey(length int) string {
	return p.keyPrefix + fmt.Sprintf(lockKeyFormat, length)
}

// Pull implements cores.ShortCodePool.Pull
func (p *RedisShortCodePool) Pull(ctx context.Context, length int) (string, bool, error) {
	// Use ZPOPMIN to get the earliest added short code
	result := p.redis.ZPopMin(ctx, p.getPoolKey(length))
	if result.Err() != nil {
		return "", false, result.Err()
	}

	// Check if we got a result
	values, err := result.Result()
	if err != nil {
		return "", false, err
	}

	if len(values) == 0 {
		return "", false, nil
	}

	// Get the short code
	code := values[0].Member.(string)

	// Check if the pool is running low
	size, err := p.Size(ctx, length)
	if err != nil {
		return code, false, nil
	}

	enough := size > 100

	return code, enough, nil
}

// Add implements cores.ShortCodePool.Add
func (p *RedisShortCodePool) Add(ctx context.Context, length int, shortCode string) error {
	result := p.redis.ZAdd(ctx, p.getPoolKey(length), redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: shortCode,
	})

	if result.Err() != nil {
		return result.Err()
	}

	return nil
}

// Size implements cores.ShortCodePool.Size
func (p *RedisShortCodePool) Size(ctx context.Context, length int) (int64, error) {
	// Use ZCARD to get the number of short codes in the pool
	result := p.redis.ZCard(ctx, p.getPoolKey(length))
	if result.Err() != nil {
		return 0, result.Err()
	}

	return result.Result()
}

// Lock implements cores.ShortCodePool.Lock
func (p *RedisShortCodePool) Lock(ctx context.Context, length int, expire time.Duration) error {
	// Use SET with NX option to implement a lock
	result := p.redis.SetNX(ctx, p.getLockKey(length), strconv.FormatInt(time.Now().Add(expire).Unix(), 10), expire)
	if result.Err() != nil {
		return result.Err()
	}

	// Check if we got the lock
	success, err := result.Result()
	if err != nil {
		return err
	}

	if !success {
		// Check if the lock is expired
		valueResult := p.redis.Get(ctx, p.getLockKey(length))
		if valueResult.Err() != nil {
			if valueResult.Err() == redis.Nil {
				// Lock key doesn't exist, try to lock again
				return p.Lock(ctx, length, expire)
			}
			return valueResult.Err()
		}

		// Get the expiration time
		expireTimeStr, err := valueResult.Result()
		if err != nil {
			return err
		}

		expireTime, err := strconv.ParseInt(expireTimeStr, 10, 64)
		if err != nil {
			return err
		}

		// Check if the lock is expired
		if time.Now().Unix() > expireTime {
			// Lock is expired, delete it and try to lock again
			p.redis.Del(ctx, p.getLockKey(length))
			return p.Lock(ctx, length, expire)
		}

		return ErrPoolLocked
	}

	return nil
}

// Unlock implements cores.ShortCodePool.Unlock
func (p *RedisShortCodePool) Unlock(ctx context.Context, length int) {
	p.redis.Del(ctx, p.getLockKey(length))
}

// Remove implements cores.ShortCodePool.Remove
func (p *RedisShortCodePool) Remove(ctx context.Context, length int, shortCode string) error {
	// Use ZREM to remove the specific short code from the pool
	result := p.redis.ZRem(ctx, p.getPoolKey(length), shortCode)
	if result.Err() != nil {
		return result.Err()
	}

	return nil
}

// Clear implements cores.ShortCodePool.Clear
func (p *RedisShortCodePool) Clear(ctx context.Context, length int) error {
	// Use DEL to remove the entire pool key
	result := p.redis.Del(ctx, p.getPoolKey(length))
	if result.Err() != nil {
		return result.Err()
	}

	return nil
}

// Close implements cores.ShortCodePool.Close
func (p *RedisShortCodePool) Close(ctx context.Context) error {
	// Note: We don't close the Redis client as it's typically managed by the application
	return nil
}
