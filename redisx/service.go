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

package redisx

import (
	"github.com/redis/go-redis/v9"
	"github.com/vogo/vshortlink/cores"
)

// RedisShortLinkService implements the ShortLinkService interface using Redis
type RedisShortLinkService struct {
	*cores.ShortLinkService
	redis *redis.Client
}

// NewRedisShortLinkService creates a new RedisShortLinkService
func NewRedisShortLinkService(redisClient *redis.Client, opts ...cores.ServiceOption) *RedisShortLinkService {
	// Create Redis repository
	repo := NewRedisShortLinkRepository(redisClient)

	// Create Redis cache
	cache := NewRedisShortLinkCache(redisClient)

	// Create Redis pool
	pool := NewRedisShortCodePool(redisClient)

	// Create core service
	coreService := cores.NewShortLinkService(repo, cache, pool, opts...)

	return &RedisShortLinkService{
		ShortLinkService: coreService,
		redis:            redisClient,
	}
}
