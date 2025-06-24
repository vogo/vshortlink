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

package memx

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/vogo/vshortlink/cores"
)

// Error definitions
var (
	// ErrLinkNotFound is returned when a short link is not found
	ErrLinkNotFound = errors.New("short link not found")
)

// MemoryShortLinkRepository implements cores.ShortLinkRepository interface with in-memory storage
type MemoryShortLinkRepository struct {
	mutex      sync.RWMutex
	links      map[int64]*cores.ShortLink // ID -> ShortLink
	codeToID   map[string]int64           // Code -> ID
	startIndex map[int]int64              // Length -> StartIndex
	nextID     int64
}

// NewMemoryShortLinkRepository creates a new MemoryShortLinkRepository
func NewMemoryShortLinkRepository() *MemoryShortLinkRepository {
	return &MemoryShortLinkRepository{
		links:      make(map[int64]*cores.ShortLink),
		codeToID:   make(map[string]int64),
		startIndex: make(map[int]int64),
		nextID:     1,
	}
}

// Create implements cores.ShortLinkRepository.Create
func (r *MemoryShortLinkRepository) Create(ctx context.Context, link *cores.ShortLink) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	link.ID = r.nextID
	r.nextID++

	r.links[link.ID] = link
	r.codeToID[link.Code] = link.ID

	return nil
}

// Update implements cores.ShortLinkRepository.Update
func (r *MemoryShortLinkRepository) Update(ctx context.Context, link *cores.ShortLink) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.links[link.ID]; !exists {
		return ErrLinkNotFound
	}

	r.links[link.ID] = link
	r.codeToID[link.Code] = link.ID

	return nil
}

// Updates implements cores.ShortLinkRepository.Updates
func (r *MemoryShortLinkRepository) Updates(ctx context.Context, links []*cores.ShortLink) error {
	for _, link := range links {
		if err := r.Update(ctx, link); err != nil {
			return err
		}
	}

	return nil
}

// Delete implements cores.ShortLinkRepository.Delete
func (r *MemoryShortLinkRepository) Delete(ctx context.Context, id int64) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	link, exists := r.links[id]
	if !exists {
		return ErrLinkNotFound
	}

	delete(r.codeToID, link.Code)
	delete(r.links, id)

	return nil
}

// GetByCode implements cores.ShortLinkRepository.GetByCode
func (r *MemoryShortLinkRepository) GetByCode(ctx context.Context, code string) (*cores.ShortLink, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	id, exists := r.codeToID[code]
	if !exists {
		return nil, ErrLinkNotFound
	}

	return r.links[id], nil
}

// GetByID implements cores.ShortLinkRepository.GetByID
func (r *MemoryShortLinkRepository) GetByID(ctx context.Context, id int64) (*cores.ShortLink, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	link, exists := r.links[id]
	if !exists {
		return nil, ErrLinkNotFound
	}

	return link, nil
}

// FindExpiredActives implements cores.ShortLinkRepository.FindExpiredActives
func (r *MemoryShortLinkRepository) FindExpiredActives(ctx context.Context, fromID int64, limit int) ([]*cores.ShortLink, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var result []*cores.ShortLink
	now := time.Now()

	for id, link := range r.links {
		if id <= fromID {
			continue
		}

		if link.IsActive() && link.Expire.Before(now) {
			result = append(result, link)
		}

		if len(result) >= limit {
			break
		}
	}

	return result, nil
}

// FindExpires implements cores.ShortLinkRepository.FindExpires
func (r *MemoryShortLinkRepository) FindExpires(ctx context.Context, fromID int64, expiredBefore time.Time, limit int) ([]*cores.ShortLink, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var result []*cores.ShortLink

	for id, link := range r.links {
		if id <= fromID {
			continue
		}

		if link.IsExpired() && link.Expire.Before(expiredBefore) {
			result = append(result, link)
		}

		if len(result) >= limit {
			break
		}
	}

	return result, nil
}

// GetStartIndex implements cores.ShortLinkRepository.GetStartIndex
func (r *MemoryShortLinkRepository) GetStartIndex(ctx context.Context, length int) (int64, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	index, exists := r.startIndex[length]
	if !exists {
		return 0, nil
	}

	return index, nil
}

// SaveStartIndex implements cores.ShortLinkRepository.SaveStartIndex
func (r *MemoryShortLinkRepository) SaveStartIndex(ctx context.Context, length int, index int64) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.startIndex[length] = index

	return nil
}
