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
	"sync"
	"time"
)

// MemoryShortCodePool implements cores.ShortCodePool interface with in-memory storage
type MemoryShortCodePool struct {
	mutex sync.RWMutex
	// map[length][]code
	pools      map[int][]string
	lockStatus map[int]time.Time
}

// NewMemoryShortCodePool creates a new MemoryShortCodePool
func NewMemoryShortCodePool() *MemoryShortCodePool {
	return &MemoryShortCodePool{
		pools:      make(map[int][]string),
		lockStatus: make(map[int]time.Time),
	}
}

// Pull implements cores.ShortCodePool.Pull
func (p *MemoryShortCodePool) Pull(ctx context.Context, length int) (string, bool, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	codes, exists := p.pools[length]
	if !exists || len(codes) == 0 {
		return "", false, nil
	}

	// Get the first code from the pool
	code := codes[0]

	// Remove the code from the pool
	p.pools[length] = codes[1:]

	// Check if the pool is running low
	enough := len(p.pools[length]) > 10

	return code, enough, nil
}

// Add implements cores.ShortCodePool.Add
func (p *MemoryShortCodePool) Add(ctx context.Context, length int, shortCode string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Initialize the pool if it doesn't exist
	_, exists := p.pools[length]
	if !exists {
		p.pools[length] = make([]string, 0)
	}

	// Add the code to the pool
	p.pools[length] = append(p.pools[length], shortCode)

	return nil
}

// Size implements cores.ShortCodePool.Size
func (p *MemoryShortCodePool) Size(ctx context.Context, length int) (int64, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	codes, exists := p.pools[length]
	if !exists {
		return 0, nil
	}

	return int64(len(codes)), nil
}

// Lock implements cores.ShortCodePool.Lock
func (p *MemoryShortCodePool) Lock(ctx context.Context, length int, expire time.Duration) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Check if the pool is already locked and not expired
	lockTime, exists := p.lockStatus[length]
	if exists && time.Now().Before(lockTime) {
		return nil
	}

	// Lock the pool
	p.lockStatus[length] = time.Now().Add(expire)

	return nil
}

// Unlock implements cores.ShortCodePool.Unlock
func (p *MemoryShortCodePool) Unlock(ctx context.Context, length int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Unlock the pool
	delete(p.lockStatus, length)
}
