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
		Password: "", // 无密码
		DB:       0,  // 使用默认DB
	})

	ctx := context.Background()
	pong, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("无法连接到Redis: %v", err)
	}
	log.Printf("Redis连接成功: %s", pong)

	repo := memx.NewMemoryShortLinkRepository()
	cache := redisx.NewRedisShortLinkCache(redisClient)
	pool := redisx.NewRedisShortCodePool(redisClient)
	service := cores.NewShortLinkService(repo, cache, pool)
	defer service.Close()

	link, err := service.Create(ctx, "https://example.com", 3, time.Now().Add(time.Minute))
	if err != nil {
		log.Fatalf("创建短链接失败: %v", err)
	}

	log.Printf("创建短链接成功: %+v", link)

	foundLink, err := service.Repo.GetByCode(ctx, link.Code)
	if err != nil {
		log.Fatalf("获取短链接失败: %v", err)
	}

	log.Printf("获取短链接成功: %+v", foundLink)

	log.Println("等待短链接过期...")
	time.Sleep(time.Minute + time.Second)

	log.Println("处理过期的链接...")
	service.ExpireActives()
	log.Println("处理过期链接完成")

	log.Println("回收过期的链接...")
	service.RecycleExpires()
	log.Println("回收过期链接完成")

	newLink, err := service.Create(ctx, "https://example.org", 3, time.Now().Add(time.Hour))
	if err != nil {
		log.Fatalf("创建新短链接失败: %v", err)
	}

	log.Printf("创建新短链接成功: %+v", newLink)
	log.Printf("新短链接是否重用了回收的短码: %v", newLink.Code == link.Code)

	fmt.Println("示例运行完成")
}
