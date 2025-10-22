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
	"time"

	"github.com/vogo/vshortlink/cores"
	"github.com/vogo/vshortlink/memx"
)

func main() {
	repo := memx.NewMemoryShortLinkRepository()
	cache := memx.NewMemoryShortLinkCache()
	pool := memx.NewMemoryShortCodePool()

	// Create the core service
	service := cores.NewShortLinkService(repo, cache, pool,
		cores.WithBatchGenerateSize(100),
		cores.WithMaxCodeLength(6))
	defer service.Close()

	// Create a new short link
	ctx := context.Background()
	_, err := service.Create(ctx, "title1", "https://example.com", 4, time.Now().Add(24*time.Hour))
	if err != nil {
		fmt.Printf("Failed to create short link: %v\n", err)
		return
	}
	link, err := service.Create(ctx, "title2", "https://example.com", 4, time.Now().Add(24*time.Hour))
	if err != nil {
		fmt.Printf("Failed to create short link: %v\n", err)
		return
	}

	// Print the short link
	fmt.Printf("Created short link: %s -> %s (expires: %s)\n", link.Code, link.Link, link.Expire.Format(time.RFC3339))

	// Create another short link with a different length
	link2, err := service.Create(ctx, "title3", "https://another-example.com", 5, time.Now().Add(48*time.Hour))
	if err != nil {
		fmt.Printf("Failed to create second short link: %v\n", err)
		return
	}

	// Print the second short link
	fmt.Printf("Created second short link: %s -> %s (expires: %s)\n", link2.Code, link2.Link, link2.Expire.Format(time.RFC3339))

	// Demonstrate expiring active links
	fmt.Println("\nExpiring active links...")
	// Set the first link to expire in the past
	link.Expire = time.Now().Add(-1 * time.Hour)
	err = service.Repo.Update(ctx, link)
	if err != nil {
		fmt.Printf("Failed to update link expiration: %v\n", err)
		return
	}

	// Run the expire actives process
	service.ExpireActives()

	// Try to get the expired link from cache
	_, found := service.Cache.Get(ctx, link.Length, link.Code)
	fmt.Printf("Expired link in cache: %v\n", found)

	// Demonstrate recycling expired links
	fmt.Println("\nRecycling expired links...")
	// Set the expired link to have expired a year ago
	link.Expire = time.Now().Add(-365 * 24 * time.Hour)
	err = service.Repo.Update(ctx, link)
	if err != nil {
		fmt.Printf("Failed to update link expiration for recycling: %v\n", err)
		return
	}

	// Run the recycle expires process
	service.RecycleExpires()

	// Check if the code was added back to the pool
	poolSize, _ := service.Pool.Size(ctx, link.Length)
	fmt.Printf("Pool size for length %d: %d\n", link.Length, poolSize)

	// Create a new link with the same length to see if we get the recycled code
	link3, err := service.Create(ctx, "title4", "https://recycled-example.com", link.Length, time.Now().Add(24*time.Hour))
	if err != nil {
		fmt.Printf("Failed to create link with recycled code: %v\n", err)
		return
	}

	// Print the new link
	fmt.Printf("Created link with recycled code: %s -> %s (expires: %s)\n", link3.Code, link3.Link, link3.Expire.Format(time.RFC3339))
}
