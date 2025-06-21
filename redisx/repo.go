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
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vogo/vshortlink/cores"
)

const (
	// Redis key prefixes
	linkKeyPrefix       = "shortlink:link"          // Hash storing all links by ID
	codeToIDKeyPrefix   = "shortlink:code2id"       // Hash mapping code to ID
	startIndexKeyPrefix = "shortlink:startindex:%d" // Key storing start index for each length
	activeZSetKey       = "shortlink:active"        // ZSet of active links sorted by ID
	expiredZSetKey      = "shortlink:expired"       // ZSet of expired links sorted by ID
	nextIDKey           = "shortlink:nextid"        // Key storing the next available ID
)

// RedisShortLinkRepository implements cores.ShortLinkRepository interface with Redis storage
type RedisShortLinkRepository struct {
	redis   *redis.Client
	idMutex sync.Mutex // Mutex to protect ID generation
}

// NewRedisShortLinkRepository creates a new RedisShortLinkRepository
func NewRedisShortLinkRepository(redisClient *redis.Client) *RedisShortLinkRepository {
	return &RedisShortLinkRepository{
		redis: redisClient,
	}
}

// getNextID gets the next available ID and increments it
func (r *RedisShortLinkRepository) getNextID(ctx context.Context) (int64, error) {
	r.idMutex.Lock()
	defer r.idMutex.Unlock()

	// Use INCR to get the next ID
	result := r.redis.Incr(ctx, nextIDKey)
	if result.Err() != nil {
		return 0, result.Err()
	}

	return result.Result()
}

// Create implements cores.ShortLinkRepository.Create
func (r *RedisShortLinkRepository) Create(ctx context.Context, link *cores.ShortLink) error {
	// Get the next ID
	id, err := r.getNextID(ctx)
	if err != nil {
		return err
	}

	// Set the ID
	link.ID = id

	// Serialize the link
	linkJSON, err := json.Marshal(link)
	if err != nil {
		return err
	}

	// Use a pipeline to perform multiple operations atomically
	pipe := r.redis.Pipeline()

	// Store the link
	pipe.HSet(ctx, linkKeyPrefix, strconv.FormatInt(link.ID, 10), linkJSON)

	// Map code to ID
	pipe.HSet(ctx, codeToIDKeyPrefix, link.Code, link.ID)

	// Add to active ZSet if active
	if link.IsActive() {
		pipe.ZAdd(ctx, activeZSetKey, redis.Z{
			Score:  float64(link.ID),
			Member: link.ID,
		})
	}

	// Execute the pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Update implements cores.ShortLinkRepository.Update
func (r *RedisShortLinkRepository) Update(ctx context.Context, link *cores.ShortLink) error {
	// Check if the link exists
	exists, err := r.redis.HExists(ctx, linkKeyPrefix, strconv.FormatInt(link.ID, 10)).Result()
	if err != nil {
		return err
	}

	if !exists {
		return ErrLinkNotFound
	}

	// Serialize the link
	linkJSON, err := json.Marshal(link)
	if err != nil {
		return err
	}

	// Use a pipeline to perform multiple operations atomically
	pipe := r.redis.Pipeline()

	// Update the link
	pipe.HSet(ctx, linkKeyPrefix, strconv.FormatInt(link.ID, 10), linkJSON)

	// Update ZSets based on status
	if link.IsActive() {
		// Add to active ZSet, remove from expired ZSet
		pipe.ZAdd(ctx, activeZSetKey, redis.Z{
			Score:  float64(link.ID),
			Member: link.ID,
		})
		pipe.ZRem(ctx, expiredZSetKey, link.ID)
	} else if link.IsExpired() {
		// Add to expired ZSet, remove from active ZSet
		pipe.ZAdd(ctx, expiredZSetKey, redis.Z{
			Score:  float64(link.ID),
			Member: link.ID,
		})
		pipe.ZRem(ctx, activeZSetKey, link.ID)
	} else if link.IsRecycle() {
		// Remove from both ZSets
		pipe.ZRem(ctx, activeZSetKey, link.ID)
		pipe.ZRem(ctx, expiredZSetKey, link.ID)
	}

	// Execute the pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

// Updates implements cores.ShortLinkRepository.Updates
func (r *RedisShortLinkRepository) Updates(ctx context.Context, links []*cores.ShortLink) error {
	for _, link := range links {
		if err := r.Update(ctx, link); err != nil {
			return err
		}
	}

	return nil
}

// Delete implements cores.ShortLinkRepository.Delete
func (r *RedisShortLinkRepository) Delete(ctx context.Context, id int64) error {
	// Get the link first to get the code
	link, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Use a pipeline to perform multiple operations atomically
	pipe := r.redis.Pipeline()

	// Delete the link
	pipe.HDel(ctx, linkKeyPrefix, strconv.FormatInt(id, 10))

	// Delete the code to ID mapping
	pipe.HDel(ctx, codeToIDKeyPrefix, link.Code)

	// Remove from ZSets
	pipe.ZRem(ctx, activeZSetKey, id)
	pipe.ZRem(ctx, expiredZSetKey, id)

	// Execute the pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

// GetByCode implements cores.ShortLinkRepository.GetByCode
func (r *RedisShortLinkRepository) GetByCode(ctx context.Context, code string) (*cores.ShortLink, error) {
	// Get the ID from the code
	idResult := r.redis.HGet(ctx, codeToIDKeyPrefix, code)
	if idResult.Err() != nil {
		if idResult.Err() == redis.Nil {
			return nil, ErrLinkNotFound
		}
		return nil, idResult.Err()
	}

	idStr, err := idResult.Result()
	if err != nil {
		return nil, err
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, err
	}

	// Get the link by ID
	return r.GetByID(ctx, id)
}

// GetByID implements cores.ShortLinkRepository.GetByID
func (r *RedisShortLinkRepository) GetByID(ctx context.Context, id int64) (*cores.ShortLink, error) {
	// Get the link from Redis
	result := r.redis.HGet(ctx, linkKeyPrefix, strconv.FormatInt(id, 10))
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return nil, ErrLinkNotFound
		}
		return nil, result.Err()
	}

	linkJSON, err := result.Result()
	if err != nil {
		return nil, err
	}

	// Deserialize the link
	var link cores.ShortLink
	if err := json.Unmarshal([]byte(linkJSON), &link); err != nil {
		return nil, err
	}

	return &link, nil
}

// FindExpiredActives implements cores.ShortLinkRepository.FindExpiredActives
func (r *RedisShortLinkRepository) FindExpiredActives(ctx context.Context, fromID int64, limit int) ([]*cores.ShortLink, error) {
	// Get active links with ID > fromID
	result := r.redis.ZRangeByScore(ctx, activeZSetKey, &redis.ZRangeBy{
		Min:    fmt.Sprintf("(%d", fromID), // Exclusive range
		Max:    "+inf",
		Offset: 0,
		Count:  int64(limit),
	})

	if result.Err() != nil {
		return nil, result.Err()
	}

	ids, err := result.Result()
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return []*cores.ShortLink{}, nil
	}

	// Get the links
	var links []*cores.ShortLink
	now := time.Now()

	for _, idStr := range ids {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}

		link, err := r.GetByID(ctx, id)
		if err != nil {
			continue
		}

		// Check if the link is active and expired
		if link.IsActive() && link.Expire.Before(now) {
			links = append(links, link)
		}

		if len(links) >= limit {
			break
		}
	}

	return links, nil
}

// FindExpires implements cores.ShortLinkRepository.FindExpires
func (r *RedisShortLinkRepository) FindExpires(ctx context.Context, fromID int64, expiredBefore time.Time, limit int) ([]*cores.ShortLink, error) {
	// Get expired links with ID > fromID
	result := r.redis.ZRangeByScore(ctx, expiredZSetKey, &redis.ZRangeBy{
		Min:    fmt.Sprintf("(%d", fromID), // Exclusive range
		Max:    "+inf",
		Offset: 0,
		Count:  int64(limit),
	})

	if result.Err() != nil {
		return nil, result.Err()
	}

	ids, err := result.Result()
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return []*cores.ShortLink{}, nil
	}

	// Get the links
	var links []*cores.ShortLink

	for _, idStr := range ids {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}

		link, err := r.GetByID(ctx, id)
		if err != nil {
			continue
		}

		// Check if the link is expired and expired before the given time
		if link.IsExpired() && link.Expire.Before(expiredBefore) {
			links = append(links, link)
		}

		if len(links) >= limit {
			break
		}
	}

	return links, nil
}

// GetStartIndex implements cores.ShortLinkRepository.GetStartIndex
func (r *RedisShortLinkRepository) GetStartIndex(ctx context.Context, length int) (int64, error) {
	// Get the start index from Redis
	result := r.redis.Get(ctx, fmt.Sprintf(startIndexKeyPrefix, length))
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return 0, nil
		}
		return 0, result.Err()
	}

	indexStr, err := result.Result()
	if err != nil {
		return 0, err
	}

	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return index, nil
}

// SaveStartIndex implements cores.ShortLinkRepository.SaveStartIndex
func (r *RedisShortLinkRepository) SaveStartIndex(ctx context.Context, length int, index int64) error {
	// Save the start index to Redis
	result := r.redis.Set(ctx, fmt.Sprintf(startIndexKeyPrefix, length), index, 0)
	if result.Err() != nil {
		return result.Err()
	}

	return nil
}
