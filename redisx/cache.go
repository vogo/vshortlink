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
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis cache key format
	cacheKeyFormat = "shortlink:cache:%d" // shortlink:cache:{length}
)

// RedisShortLinkCache implements cores.ShortLinkCache interface with Redis storage
type RedisShortLinkCache struct {
	redis     *redis.Client
	keyPrefix string
}

type CacheOption func(c *RedisShortLinkCache)

func WithCacheKeyPrefix(prefix string) CacheOption {
	return func(c *RedisShortLinkCache) {
		c.keyPrefix = prefix
	}
}

// NewRedisShortLinkCache creates a new RedisShortLinkCache
func NewRedisShortLinkCache(redisClient *redis.Client, opts ...CacheOption) *RedisShortLinkCache {
	c := &RedisShortLinkCache{
		redis: redisClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// getCacheKey returns the Redis key for the cache of the given length
func (c *RedisShortLinkCache) getCacheKey(length int) string {
	return c.keyPrefix + fmt.Sprintf(cacheKeyFormat, length)
}

// Get implements cores.ShortLinkCache.Get
func (c *RedisShortLinkCache) Get(ctx context.Context, length int, code string) (string, bool) {
	result := c.redis.HGet(ctx, c.getCacheKey(length), code)
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return "", false
		}
		return "", false
	}

	value, err := result.Result()
	if err != nil {
		return "", false
	}

	parts := splitLinkAndExpireTime(value)
	if len(parts) != 2 {
		return "", false
	}

	link := parts[0]
	expireTimeUnix, err := parseExpireTime(parts[1])
	if err != nil {
		return "", false
	}

	if time.Now().After(time.Unix(expireTimeUnix, 0)) {
		_ = c.Remove(ctx, length, code)
		return "", false
	}

	return link, true
}

// Add implements cores.ShortLinkCache.Add
func (c *RedisShortLinkCache) Add(ctx context.Context, length int, code string, link string, expireTime time.Time) error {
	value := formatLinkWithExpireTime(link, expireTime)

	result := c.redis.HSet(ctx, c.getCacheKey(length), code, value)
	if result.Err() != nil {
		return result.Err()
	}

	return nil
}

// Remove implements cores.ShortLinkCache.Remove
func (c *RedisShortLinkCache) Remove(ctx context.Context, length int, code string) error {
	result := c.redis.HDel(ctx, c.getCacheKey(length), code)
	if result.Err() != nil {
		return result.Err()
	}

	return nil
}

// Close implements cores.ShortLinkCache.Close
func (c *RedisShortLinkCache) Close(ctx context.Context) error {
	// Note: We don't close the Redis client as it's typically managed by the application
	return nil
}

// formatLinkWithExpireTime 将link和expireTime合并为一个字符串
// 格式为：link\nexpireTime，其中expireTime为Unix时间戳
func formatLinkWithExpireTime(link string, expireTime time.Time) string {
	return link + "\n" + strconv.FormatInt(expireTime.Unix(), 10)
}

// splitLinkAndExpireTime 将合并的值分割为link和expireTime
func splitLinkAndExpireTime(value string) []string {
	return strings.SplitN(value, "\n", 2)
}

// parseExpireTime 解析过期时间字符串为Unix时间戳
func parseExpireTime(expireTimeStr string) (int64, error) {
	return strconv.ParseInt(expireTimeStr, 10, 64)
}
