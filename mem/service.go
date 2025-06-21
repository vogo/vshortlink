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

package mem

import (
	"github.com/vogo/vshortlink/cores"
)

// MemoryShortLinkService is a memory-based implementation of ShortLinkService
type MemoryShortLinkService struct {
	*cores.ShortLinkService
	Repo  *MemoryShortLinkRepository
	Cache *MemoryShortLinkCache
	Pool  *MemoryShortCodePool
}

// NewMemoryShortLinkService creates a new MemoryShortLinkService with in-memory implementations
// of repository, cache, and pool
func NewMemoryShortLinkService(batchGenerateSize int64, maxCodeLength int) *MemoryShortLinkService {
	// Create memory-based implementations
	repo := NewMemoryShortLinkRepository()
	cache := NewMemoryShortLinkCache()
	pool := NewMemoryShortCodePool()

	// Create the core service
	coreService := cores.NewShortLinkService(repo, cache, pool, batchGenerateSize, maxCodeLength)

	// Return the memory service
	return &MemoryShortLinkService{
		ShortLinkService: coreService,
		Repo:             repo,
		Cache:            cache,
		Pool:             pool,
	}
}
