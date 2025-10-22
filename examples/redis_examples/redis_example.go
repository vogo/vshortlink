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

package examples

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vogo/vshortlink/cores"
	"github.com/vogo/vshortlink/memx"
	"github.com/vogo/vshortlink/redisx"
)

// RedisExample demonstrates how to use the Redis-based short link service
func RedisExample() {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // No password
		DB:       0,  // Use default DB
	})

	ctx := context.Background()
	pong, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Unable to connect to Redis: %v", err)
	}
	log.Printf("Redis connection successful: %s", pong)

	repo := memx.NewMemoryShortLinkRepository()
	cache := redisx.NewRedisShortLinkCache(redisClient)
	pool := redisx.NewRedisShortCodePool(redisClient)
	service := cores.NewShortLinkService(repo, cache, pool)
	defer service.Close()

	link, err := service.Create(ctx, "title1", "https://example.com", 3, time.Now().Add(time.Minute))
	if err != nil {
		log.Fatalf("Failed to create short link: %v", err)
	}

	log.Printf("Short link created successfully: %+v", link)

	foundLink, err := service.Repo.GetByCode(ctx, link.Code)
	if err != nil {
		log.Fatalf("Failed to get short link: %v", err)
	}

	log.Printf("Short link retrieved successfully: %+v", foundLink)

	log.Println("Waiting for short link to expire...")
	time.Sleep(time.Minute + time.Second)

	log.Println("Processing expired links...")
	service.ExpireActives()
	log.Println("Expired link processing completed")

	log.Println("Recycling expired links...")
	service.RecycleExpires()
	log.Println("Expired link recycling completed")

	newLink, err := service.Create(ctx, "title2", "https://example.org", 3, time.Now().Add(time.Hour))
	if err != nil {
		log.Fatalf("Failed to create new short link: %v", err)
	}

	log.Printf("New short link created successfully: %+v", newLink)
	log.Printf("Whether the new short link reused the recycled short code: %v", newLink.Code == link.Code)

	fmt.Println("Example execution completed")
}
