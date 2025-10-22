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

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vogo/vshortlink/cores"
	"github.com/vogo/vshortlink/memx"
)

func main() {
	// Create memory-based Repository, Cache and Pool
	repo := memx.NewMemoryShortLinkRepository()
	cache := memx.NewMemoryShortLinkCache()
	pool := memx.NewMemoryShortCodePool()

	// Create ShortLinkService
	service := cores.NewShortLinkService(repo, cache, pool,
		cores.WithMaxCodeLength(6),
		cores.WithBatchGenerateSize(100),
	)
	defer service.Close()

	ctx := context.Background()

	// 1. Create some short links to simulate used short codes
	fmt.Println("=== Creating test short links ===")
	for i := 0; i < 5; i++ {
		link, err := service.Create(ctx, fmt.Sprintf("title%d", i), fmt.Sprintf("https://example.com/%d", i), 4, time.Now().Add(time.Hour*24))
		if err != nil {
			log.Fatalf("Failed to create short link: %v", err)
		}
		fmt.Printf("Created short link: %s -> %s\n", link.Code, link.Link)
	}

	// 2. Check short code pool status
	fmt.Println("\n=== Short code pool status before rebuild ===")
	poolSize, err := pool.Size(ctx, 4)
	if err != nil {
		log.Fatalf("Failed to get short code pool size: %v", err)
	}
	fmt.Printf("Short code pool size for length 4: %d\n", poolSize)

	// 3. Simulate short code pool data loss (clear pool)
	fmt.Println("\n=== Simulating short code pool data loss ===")
	err = pool.Clear(ctx, 4)
	if err != nil {
		log.Fatalf("Failed to clear short code pool: %v", err)
	}
	poolSize, _ = pool.Size(ctx, 4)
	fmt.Printf("Short code pool size after clearing: %d\n", poolSize)

	// 4. Use RebuildCodePool to rebuild the short code pool
	fmt.Println("\n=== Rebuilding short code pool ===")
	err = service.RebuildCodePool(ctx, 4)
	if err != nil {
		log.Fatalf("Failed to rebuild short code pool: %v", err)
	}

	// 5. Check short code pool status after rebuild
	fmt.Println("\n=== Short code pool status after rebuild ===")
	poolSize, err = pool.Size(ctx, 4)
	if err != nil {
		log.Fatalf("Failed to get short code pool size: %v", err)
	}
	fmt.Printf("Short code pool size after rebuild: %d\n", poolSize)

	// 6. Test pulling short codes from the rebuilt pool
	fmt.Println("\n=== Testing pulling short codes from rebuilt pool ===")
	for i := 0; i < 3; i++ {
		code, enough, err := pool.Pull(ctx, 4)
		if err != nil {
			log.Fatalf("Failed to pull short code: %v", err)
		}
		fmt.Printf("Pulled short code: %s, enough short codes remaining in pool: %t\n", code, enough)
	}

	fmt.Println("\n=== Rebuild short code pool example completed ===")
}
