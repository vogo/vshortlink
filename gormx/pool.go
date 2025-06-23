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
	"context"
	"sync"
	"time"

	"gorm.io/gorm"
)

// GormShortCodePool implements cores.ShortCodePool interface with GORM
type GormShortCodePool struct {
	db    *gorm.DB
	mutex sync.Mutex
	// In-memory pool for better performance
	pools map[int][]string
}

// PoolCodeModel is the GORM model for short code pool
type PoolCodeModel struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	Length    int    `gorm:"index"`
	Code      string `gorm:"size:32"`
	CreatedAt time.Time
}

// TableName returns the table name for the PoolCodeModel
func (PoolCodeModel) TableName() string {
	return "short_link_pool_codes"
}

// NewGormShortCodePool creates a new GormShortCodePool
func NewGormShortCodePool(db *gorm.DB) *GormShortCodePool {
	return &GormShortCodePool{
		db:    db,
		pools: make(map[int][]string),
	}
}

// Pull implements cores.ShortCodePool.Pull
func (p *GormShortCodePool) Pull(ctx context.Context, length int) (string, bool, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Check if we have codes in memory
	codes, exists := p.pools[length]
	if exists && len(codes) > 0 {
		// Get the first code from the pool
		code := codes[0]

		// Remove the code from the pool
		p.pools[length] = codes[1:]

		// Delete from database
		p.db.WithContext(ctx).Where("length = ? AND code = ?", length, code).Delete(&PoolCodeModel{})

		// Check if the pool is running low
		enough := len(p.pools[length]) > 10

		return code, enough, nil
	}

	// If not in memory, try to fetch from database
	var model PoolCodeModel
	result := p.db.WithContext(ctx).Where("length = ?", length).Order("created_at ASC").First(&model)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return "", false, nil
		}
		return "", false, result.Error
	}

	// Delete the code from the database
	p.db.WithContext(ctx).Delete(&model)

	// Check if the pool is running low by counting remaining codes
	var count int64
	p.db.WithContext(ctx).Model(&PoolCodeModel{}).Where("length = ?", length).Count(&count)

	enough := count > 10

	return model.Code, enough, nil
}

// Add implements cores.ShortCodePool.Add
func (p *GormShortCodePool) Add(ctx context.Context, length int, shortCode string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Add to memory pool
	_, exists := p.pools[length]
	if !exists {
		p.pools[length] = make([]string, 0)
	}
	p.pools[length] = append(p.pools[length], shortCode)

	// Add to database
	model := PoolCodeModel{
		Length: length,
		Code:   shortCode,
	}

	result := p.db.WithContext(ctx).Create(&model)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// Size implements cores.ShortCodePool.Size
func (p *GormShortCodePool) Size(ctx context.Context, length int) (int64, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Count codes in memory
	codes, exists := p.pools[length]
	memCount := int64(0)
	if exists {
		memCount = int64(len(codes))
	}

	// Count codes in database
	var dbCount int64
	result := p.db.WithContext(ctx).Model(&PoolCodeModel{}).Where("length = ?", length).Count(&dbCount)
	if result.Error != nil {
		return memCount, result.Error
	}

	return memCount + dbCount, nil
}

// Lock implements cores.ShortCodePool.Lock
func (p *GormShortCodePool) Lock(ctx context.Context, length int, expire time.Duration) error {
	// Try to create a lock record
	expireTime := time.Now().Add(expire)
	result := p.db.WithContext(ctx).Exec(
		"INSERT INTO short_link_pool_locks (length, expire_at, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?) "+
			"ON DUPLICATE KEY UPDATE expire_at = IF(expire_at < NOW(), VALUES(expire_at), expire_at), updated_at = VALUES(updated_at)",
		length, expireTime, time.Now(), time.Now(),
	)

	if result.Error != nil {
		return result.Error
	}

	// Check if we got the lock
	var lock PoolLockModel
	getResult := p.db.WithContext(ctx).Where("length = ?", length).First(&lock)
	if getResult.Error != nil {
		return getResult.Error
	}

	// If the lock is held by someone else and not expired, return error
	if lock.ExpireAt.After(time.Now()) && lock.UpdatedAt != time.Now() {
		return ErrPoolLocked
	}

	return nil
}

// Unlock implements cores.ShortCodePool.Unlock
func (p *GormShortCodePool) Unlock(ctx context.Context, length int) {
	// Delete the lock record
	p.db.WithContext(ctx).Where("length = ?", length).Delete(&PoolLockModel{})
}

// Close implements cores.ShortCodePool.Close
func (p *GormShortCodePool) Close(ctx context.Context) error {
	// Note: We don't close the DB connection as it's typically managed by the application
	return nil
}
