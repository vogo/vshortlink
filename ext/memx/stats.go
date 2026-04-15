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
)

// MemoryShortLinkStats is an in-memory cores.ShortLinkStats implementation
// intended for tests and examples. It does no automatic retention trimming;
// production deployments should use the Redis backend.
type MemoryShortLinkStats struct {
	mutex sync.RWMutex
	// day -> code -> count
	data map[string]map[string]int64
}

func NewMemoryShortLinkStats() *MemoryShortLinkStats {
	return &MemoryShortLinkStats{
		data: make(map[string]map[string]int64),
	}
}

func (s *MemoryShortLinkStats) IncrBatch(ctx context.Context, day string, deltas map[string]int64) error {
	if len(deltas) == 0 {
		return nil
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()

	codeMap, exists := s.data[day]
	if !exists {
		codeMap = make(map[string]int64, len(deltas))
		s.data[day] = codeMap
	}
	for code, delta := range deltas {
		codeMap[code] += delta
	}
	return nil
}

func (s *MemoryShortLinkStats) Get(ctx context.Context, code string, days []string) (map[string]int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	out := make(map[string]int64, len(days))
	for _, day := range days {
		codeMap, exists := s.data[day]
		if !exists {
			continue
		}
		if n, ok := codeMap[code]; ok && n != 0 {
			out[day] = n
		}
	}
	return out, nil
}

func (s *MemoryShortLinkStats) BatchGet(ctx context.Context, codes []string, days []string) (map[string]map[string]int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	out := make(map[string]map[string]int64, len(codes))
	for _, day := range days {
		codeMap, exists := s.data[day]
		if !exists {
			continue
		}
		for _, code := range codes {
			n, ok := codeMap[code]
			if !ok || n == 0 {
				continue
			}
			dayMap, exists := out[code]
			if !exists {
				dayMap = make(map[string]int64)
				out[code] = dayMap
			}
			dayMap[day] = n
		}
	}
	return out, nil
}

func (s *MemoryShortLinkStats) Close(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data = make(map[string]map[string]int64)
	return nil
}
