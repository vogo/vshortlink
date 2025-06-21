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

package gormx

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"
)

// CacheItem represents a cached short link item
type CacheItem struct {
	Link       string
	ExpireTime time.Time
}

// GormShortLinkCache implements cores.ShortLinkCache interface with in-memory storage
// Note: This implementation uses in-memory storage for caching, similar to the mem package,
// as database is not ideal for caching due to performance considerations.
type GormShortLinkCache struct {
	mutex sync.RWMutex
	// map[length]map[code]CacheItem
	cache map[int]map[string]CacheItem
	db    *gorm.DB // Kept for potential future use or extension
}

// NewGormShortLinkCache creates a new GormShortLinkCache
func NewGormShortLinkCache(db *gorm.DB) *GormShortLinkCache {
	return &GormShortLinkCache{
		cache: make(map[int]map[string]CacheItem),
		db:    db,
	}
}

// Get implements cores.ShortLinkCache.Get
func (c *GormShortLinkCache) Get(ctx context.Context, length int, code string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	codeMap, exists := c.cache[length]
	if !exists {
		return "", false
	}

	item, exists := codeMap[code]
	if !exists {
		return "", false
	}

	// Check if the item is expired
	if time.Now().After(item.ExpireTime) {
		return "", false
	}

	return item.Link, true
}

// Add implements cores.ShortLinkCache.Add
func (c *GormShortLinkCache) Add(ctx context.Context, length int, code string, link string, expireTime time.Time) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Initialize the code map if it doesn't exist
	codeMap, exists := c.cache[length]
	if !exists {
		codeMap = make(map[string]CacheItem)
		c.cache[length] = codeMap
	}

	// Add the item to the cache
	codeMap[code] = CacheItem{
		Link:       link,
		ExpireTime: expireTime,
	}

	return nil
}

// Remove implements cores.ShortLinkCache.Remove
func (c *GormShortLinkCache) Remove(ctx context.Context, length int, code string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	codeMap, exists := c.cache[length]
	if !exists {
		return nil
	}

	delete(codeMap, code)

	return nil
}
