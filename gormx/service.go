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

package gormx

import (
	"github.com/vogo/vshortlink/cores"
	"gorm.io/gorm"
)

// GormShortLinkService implements the ShortLinkService interface using GORM
type GormShortLinkService struct {
	*cores.ShortLinkService
	db *gorm.DB
}

// NewGormShortLinkService creates a new GormShortLinkService
func NewGormShortLinkService(db *gorm.DB, batchGenerateSize int64, maxCodeLength int) *GormShortLinkService {
	// Create GORM repository
	repo := NewGormShortLinkRepository(db)

	// Create cache (using in-memory implementation for better performance)
	cache := NewGormShortLinkCache(db)

	// Create pool
	pool := NewGormShortCodePool(db)

	// Create core service
	coreService := cores.NewShortLinkService(repo, cache, pool, batchGenerateSize, maxCodeLength)

	return &GormShortLinkService{
		ShortLinkService: coreService,
		db:               db,
	}
}
