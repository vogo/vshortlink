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
	// Redis key prefixes
	poolKeyPrefix = "shortlink:pool:%d"      // shortlink:pool:{length}
	lockKeyPrefix = "shortlink:pool:lock:%d" // shortlink:pool:lock:{length}
)

// RedisShortCodePool implements cores.ShortCodePool interface with Redis storage
type RedisShortCodePool struct {
	redis *redis.Client
}

// NewRedisShortCodePool creates a new RedisShortCodePool
func NewRedisShortCodePool(redisClient *redis.Client) *RedisShortCodePool {
	return &RedisShortCodePool{
		redis: redisClient,
	}
}

// getPoolKey returns the Redis key for the short code pool of the given length
func getPoolKey(length int) string {
	return fmt.Sprintf(poolKeyPrefix, length)
}

// getLockKey returns the Redis key for the lock of the short code pool of the given length
func getLockKey(length int) string {
	return fmt.Sprintf(lockKeyPrefix, length)
}

// Pull implements cores.ShortCodePool.Pull
func (p *RedisShortCodePool) Pull(ctx context.Context, length int) (string, bool, error) {
	// Use ZPOPMIN to get the earliest added short code
	result := p.redis.ZPopMin(ctx, getPoolKey(length))
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

	enough := size > 10

	return code, enough, nil
}

// Add implements cores.ShortCodePool.Add
func (p *RedisShortCodePool) Add(ctx context.Context, length int, shortCode string) error {
	// Use ZADD to add the short code to the pool with the current timestamp as score
	result := p.redis.ZAdd(ctx, getPoolKey(length), redis.Z{
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
	result := p.redis.ZCard(ctx, getPoolKey(length))
	if result.Err() != nil {
		return 0, result.Err()
	}

	return result.Result()
}

// Lock implements cores.ShortCodePool.Lock
func (p *RedisShortCodePool) Lock(ctx context.Context, length int, expire time.Duration) error {
	// Use SET with NX option to implement a lock
	result := p.redis.SetNX(ctx, getLockKey(length), strconv.FormatInt(time.Now().Add(expire).Unix(), 10), expire)
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
		valueResult := p.redis.Get(ctx, getLockKey(length))
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
			p.redis.Del(ctx, getLockKey(length))
			return p.Lock(ctx, length, expire)
		}

		return ErrPoolLocked
	}

	return nil
}

// Unlock implements cores.ShortCodePool.Unlock
func (p *RedisShortCodePool) Unlock(ctx context.Context, length int) {
	// Delete the lock key
	p.redis.Del(ctx, getLockKey(length))
}
