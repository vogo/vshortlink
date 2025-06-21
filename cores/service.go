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

	"github.com/vogo/vogo/vlog"
	"github.com/vogo/vogo/vsync/vrun"
)

type ShortLinkService struct {
	runner *vrun.Runner

	Repo  ShortLinkRepository
	Cache ShortLinkCache
	Pool  ShortCodePool

	batchGenerateSize int64
	maxCodeLength     int

	generators map[int]*ShortCodeGenerator
}

func NewShortLinkService(repo ShortLinkRepository, cache ShortLinkCache, pool ShortCodePool, batchGenerateSize int64, maxCodeLength int) *ShortLinkService {
	svc := &ShortLinkService{
		runner: vrun.New(),

		Repo:              repo,
		Cache:             cache,
		Pool:              pool,
		batchGenerateSize: batchGenerateSize,
		maxCodeLength:     maxCodeLength,
		generators:        map[int]*ShortCodeGenerator{},
	}

	ctx := context.Background()

	for i := 1; i <= maxCodeLength; i++ {
		svc.generators[i] = NewShortCodeGenerator(i, int(batchGenerateSize))
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

func (s *ShortLinkService) Stop() {
	s.runner.Stop()
}

func (s *ShortLinkService) Create(ctx context.Context, link string, shortCodeLength int, expireTime time.Time) (*ShortLink, error) {
	// 1. check link length
	if shortCodeLength < 1 || shortCodeLength > s.maxCodeLength {
		return nil, errors.New("invalid short code length")
	}

	// 2. generate short code
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

	// 3. save link
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
			link.Status = LinkStatusExpire
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
			link.Status = LinkStatusRecycle
		}

		if err = s.Repo.Updates(ctx, links); err != nil {
			vlog.Errorf("update recycle expired links failed, err: %v", err)
			return
		}

		fromId = links[len(links)-1].ID
	}
}
