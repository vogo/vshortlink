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

package cores

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/vogo/vogo/vlog"
	"github.com/vogo/vogo/vsync/vrun"
)

type ShortLinkService struct {
	runner *vrun.Runner

	Repo  ShortLinkRepository
	Cache ShortLinkCache
	Pool  ShortCodePool

	memLRUCache     *expirable.LRU[string, string]
	memLRUCacheSize int
	memLRUCacheTTL  time.Duration

	batchGenerateSize int64
	maxCodeLength     int
	authToken         string

	generators map[int]*ShortCodeGenerator
}

type ServiceOption func(*ShortLinkService)

func WithBatchGenerateSize(size int64) ServiceOption {
	return func(s *ShortLinkService) {
		s.batchGenerateSize = size
	}
}

func WithMaxCodeLength(length int) ServiceOption {
	return func(s *ShortLinkService) {
		s.maxCodeLength = length
	}
}

func WithAuthToken(token string) ServiceOption {
	return func(s *ShortLinkService) {
		s.authToken = token
	}
}

func WithRunner(runner *vrun.Runner) ServiceOption {
	return func(s *ShortLinkService) {
		s.runner = runner
	}
}

func WithMemLRUCacheSize(size int) ServiceOption {
	return func(s *ShortLinkService) {
		s.memLRUCacheSize = size
	}
}

func WithMemLRUCacheTTL(ttl time.Duration) ServiceOption {
	return func(s *ShortLinkService) {
		s.memLRUCacheTTL = ttl
	}
}

func NewShortLinkService(repo ShortLinkRepository, cache ShortLinkCache, pool ShortCodePool, opts ...ServiceOption) *ShortLinkService {
	svc := &ShortLinkService{
		runner: vrun.New(),

		Repo:              repo,
		Cache:             cache,
		Pool:              pool,
		batchGenerateSize: 100,
		maxCodeLength:     9,
		generators:        map[int]*ShortCodeGenerator{},

		memLRUCacheSize: 10240,
		memLRUCacheTTL:  time.Minute * 5,
	}

	for _, opt := range opts {
		opt(svc)
	}

	svc.memLRUCache = expirable.NewLRU[string, string](svc.memLRUCacheSize, nil, svc.memLRUCacheTTL)

	ctx := context.Background()

	for i := 1; i <= svc.maxCodeLength; i++ {
		svc.generators[i] = NewShortCodeGenerator(i, int(svc.batchGenerateSize))
		size, err := pool.Size(ctx, i)
		if err != nil {
			vlog.Panicf("get short code pool size failed, err: %v", err)
		}
		if size < 100 {
			svc.batchGenerate(ctx, i)
		}
	}

	svc.runner.Interval(svc.ExpireActives, time.Hour*24)
	svc.runner.Interval(svc.RecycleExpires, time.Hour*24)

	return svc
}

func (s *ShortLinkService) Close() {
	s.runner.Stop()

	// Close cache
	ctx := context.Background()
	if err := s.Cache.Close(ctx); err != nil {
		vlog.Errorf("close cache failed: %v", err)
	}

	// Close short code pool
	if err := s.Pool.Close(ctx); err != nil {
		vlog.Errorf("close pool failed: %v", err)
	}
}

func (s *ShortLinkService) Create(ctx context.Context, link string, shortCodeLength int, expireTime time.Time) (*ShortLink, error) {
	if shortCodeLength < 1 || shortCodeLength > s.maxCodeLength {
		return nil, errors.New("invalid short code length")
	}

	shortCode, enough, err := s.Pool.Pull(ctx, shortCodeLength)
	if err != nil {
		return nil, err
	}

	if !enough {
		go s.batchGenerate(ctx, shortCodeLength)
	}

	if err = s.Cache.Add(ctx, shortCodeLength, shortCode, link, expireTime); err != nil {
		return nil, err
	}

	shortLink := &ShortLink{
		Code:   shortCode,
		Length: shortCodeLength,
		Link:   link,
		Status: LinkStatusActive,
		Expire: expireTime,
	}

	err = s.Repo.Create(ctx, shortLink)
	if err != nil {
		return nil, err
	}

	return shortLink, nil
}

func (s *ShortLinkService) batchGenerate(ctx context.Context, shortCodeLength int) {
	err := s.Pool.Lock(ctx, shortCodeLength, time.Minute)
	if err != nil {
		vlog.Errorf("lock short code pool failed, err: %v", err)
		return
	}
	defer s.Pool.Unlock(ctx, shortCodeLength)

	startIndex, err := s.Repo.GetStartIndex(ctx, shortCodeLength)
	if err != nil {
		vlog.Errorf("get start index failed, err: %v", err)
		return
	}

	// generate batch short code
	batchNumbers, err := s.generators[shortCodeLength].GenerateBatchNumbers(shortCodeLength, startIndex)
	if err != nil {
		vlog.Errorf("generate batch short code failed, err: %v", err)
		return
	}

	// add batch short code to pool
	for _, number := range batchNumbers {
		err = s.Pool.Add(ctx, shortCodeLength, ToBase62(number, shortCodeLength))
		if err != nil {
			vlog.Errorf("add batch short code to pool failed, err: %v", err)
			return
		}
	}

	// update start index
	startIndex++
	err = s.Repo.SaveStartIndex(ctx, shortCodeLength, startIndex)
	if err != nil {
		vlog.Errorf("save start index failed, err: %v", err)
		return
	}
}

func (s *ShortLinkService) ExpireActives() {
	fromId := int64(0)
	limit := 100
	ctx := context.Background()
	for {
		links, err := s.Repo.FindExpiredActives(ctx, fromId, limit)
		if err != nil {
			vlog.Errorf("find recycle expireds failed, err: %v", err)
			return
		}

		if len(links) == 0 {
			break
		}

		for _, link := range links {
			if err = s.Cache.Remove(ctx, link.Length, link.Code); err != nil {
				vlog.Errorf("remove expired active link from cache failed, err: %v", err)
				return
			}
			link.Status = LinkStatusExpired
		}

		if err = s.Repo.Updates(ctx, links); err != nil {
			vlog.Errorf("update expired active links failed, err: %v", err)
			return
		}

		fromId = links[len(links)-1].ID
	}
}

func (s *ShortLinkService) RecycleExpires() {
	fromId := int64(0)
	limit := 100
	expiredBefore := time.Now().Add(-time.Hour * 24 * 365)

	ctx := context.Background()
	for {
		links, err := s.Repo.FindExpires(ctx, fromId, expiredBefore, limit)
		if err != nil {
			vlog.Errorf("find recycle expireds failed, err: %v", err)
			return
		}

		if len(links) == 0 {
			break
		}

		for _, link := range links {
			if err = s.Pool.Add(ctx, link.Length, link.Code); err != nil {
				vlog.Errorf("add recycle expired link to pool failed, err: %v", err)
				return
			}
			link.Status = LinkStatusRecycled
		}

		if err = s.Repo.Updates(ctx, links); err != nil {
			vlog.Errorf("update recycle expired links failed, err: %v", err)
			return
		}

		fromId = links[len(links)-1].ID
	}
}

// RebuildCodePool rebuilds the short code pool to recover data when the pool data is lost
// This method first restores all short codes to the pool in batches, then queries used codes in batches and removes them from the pool
func (s *ShortLinkService) RebuildCodePool(ctx context.Context, length int) error {
	// 1. Parameter validation
	if length < 1 || length > s.maxCodeLength {
		return errors.New("invalid short code length")
	}

	vlog.Infof("starting to rebuild code pool for length %d", length)

	// 2. Acquire pool lock to prevent concurrent operations
	err := s.Pool.Lock(ctx, length, time.Minute*5)
	if err != nil {
		return err
	}
	defer s.Pool.Unlock(ctx, length)

	// 3. Clear existing short code pool
	err = s.Pool.Clear(ctx, length)
	if err != nil {
		return err
	}
	vlog.Infof("cleared existing pool for length %d", length)

	// 4. Get current startIndex
	currentStartIndex, err := s.Repo.GetStartIndex(ctx, length)
	if err != nil {
		return err
	}
	vlog.Infof("current start index for length %d: %d", length, currentStartIndex)

	// 5. Restore all short codes to pool in batches: from startIndex=0 to current startIndex
	totalAdded := 0
	for startIndex := int64(0); startIndex < currentStartIndex; startIndex++ {
		addedCount, err := s.regenerateBatchAtIndex(ctx, length, startIndex)
		if err != nil {
			vlog.Errorf("failed to regenerate batch at index %d for length %d: %v", startIndex, length, err)
			// Continue processing next batch without interrupting the entire rebuild process
			continue
		}
		totalAdded += addedCount
	}
	vlog.Infof("regenerated %d codes for length %d from %d batches", totalAdded, length, currentStartIndex)

	// 6. Query used short codes in batches and remove them from pool
	removedCount, err := s.removeUsedCodesFromPool(ctx, length)
	if err != nil {
		return err
	}

	// 7. Record rebuild completion information
	finalPoolSize, _ := s.Pool.Size(ctx, length)
	vlog.Infof("rebuilt code pool for length %d: added %d codes, removed %d used codes, final pool size: %d",
		length, totalAdded, removedCount, finalPoolSize)

	return nil
}

// regenerateBatchAtIndex generates a batch of short codes at the specified startIndex and adds them to the pool
func (s *ShortLinkService) regenerateBatchAtIndex(ctx context.Context, length int, startIndex int64) (int, error) {
	// Generate batch short codes using the specified startIndex
	batchNumbers, err := s.generators[length].GenerateBatchNumbers(length, startIndex)
	if err != nil {
		return 0, err
	}

	// Add all generated short codes to the pool
	addedCount := 0
	for _, number := range batchNumbers {
		code := ToBase62(number, length)
		err = s.Pool.Add(ctx, length, code)
		if err != nil {
			vlog.Errorf("failed to add code %s to pool: %v", code, err)
			continue
		}
		addedCount++
	}

	vlog.Debugf("regenerated batch at startIndex %d for length %d: added %d codes", startIndex, length, addedCount)
	return addedCount, nil
}

// removeUsedCodesFromPool queries used short codes in batches and removes them from the pool
func (s *ShortLinkService) removeUsedCodesFromPool(ctx context.Context, length int) (int, error) {
	fromID := int64(0)
	limit := 1000
	totalRemoved := 0
	totalFound := 0

	// Query short links with active and expired status (not recycled)
	statuses := []LinkStatus{LinkStatusActive, LinkStatusExpired}

	for {
		links, err := s.Repo.FindByLengthAndStatus(ctx, fromID, length, statuses, limit)
		if err != nil {
			return totalRemoved, err
		}

		if len(links) == 0 {
			break
		}

		// Remove these used short codes from the pool
		batchRemoved := 0
		for _, link := range links {
			totalFound++

			err = s.Pool.Remove(ctx, length, link.Code)
			if err != nil {
				vlog.Errorf("failed to remove used code %s from pool: %v", link.Code, err)
				continue
			}

			totalRemoved++
			batchRemoved++
		}

		vlog.Debugf("found %d used codes, actually removed %d from pool for length %d (batch fromID: %d)",
			len(links), batchRemoved, length, fromID)

		fromID = links[len(links)-1].ID
	}

	vlog.Infof("found %d used codes, actually removed %d from pool for length %d",
		totalFound, totalRemoved, length)

	return totalRemoved, nil
}
