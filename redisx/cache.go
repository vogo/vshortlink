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
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes
	cacheKeyPrefix = "shortlink:cache:%d" // shortlink:cache:{length}
)

// RedisShortLinkCache implements cores.ShortLinkCache interface with Redis storage
type RedisShortLinkCache struct {
	redis *redis.Client
}

// NewRedisShortLinkCache creates a new RedisShortLinkCache
func NewRedisShortLinkCache(redisClient *redis.Client) *RedisShortLinkCache {
	return &RedisShortLinkCache{
		redis: redisClient,
	}
}

// getCacheKey returns the Redis key for the cache of the given length
func getCacheKey(length int) string {
	return fmt.Sprintf(cacheKeyPrefix, length)
}

// Get implements cores.ShortLinkCache.Get
func (c *RedisShortLinkCache) Get(ctx context.Context, length int, code string) (string, bool) {
	// Use HGET to get the link from the cache
	result := c.redis.HGet(ctx, getCacheKey(length), code)
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return "", false
		}
		return "", false
	}

	link, err := result.Result()
	if err != nil {
		return "", false
	}

	return link, true
}

// Add implements cores.ShortLinkCache.Add
func (c *RedisShortLinkCache) Add(ctx context.Context, length int, code string, link string, expireTime time.Time) error {
	// Use HSET to add the link to the cache
	result := c.redis.HSet(ctx, getCacheKey(length), code, link)
	if result.Err() != nil {
		return result.Err()
	}

	// Set expiration time for the cache key if not already set
	// Note: This sets expiration for the entire hash, not just this field
	// In a production environment, you might want to use a more sophisticated approach
	// to handle individual field expirations
	ttl := c.redis.TTL(ctx, getCacheKey(length))
	if ttl.Err() != nil {
		return ttl.Err()
	}

	ttlDuration, err := ttl.Result()
	if err != nil {
		return err
	}

	// If TTL is not set or less than the new expiration time, set it
	if ttlDuration == -1 || time.Now().Add(ttlDuration).Before(expireTime) {
		// Calculate duration until expiration
		duration := time.Until(expireTime)
		if duration > 0 {
			expireResult := c.redis.Expire(ctx, getCacheKey(length), duration)
			if expireResult.Err() != nil {
				return expireResult.Err()
			}
		}
	}

	return nil
}

// Remove implements cores.ShortLinkCache.Remove
func (c *RedisShortLinkCache) Remove(ctx context.Context, length int, code string) error {
	// Use HDEL to remove the link from the cache
	result := c.redis.HDel(ctx, getCacheKey(length), code)
	if result.Err() != nil {
		return result.Err()
	}

	return nil
}
