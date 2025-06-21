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
	"time"

	"github.com/vogo/vshortlink/cores"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormShortLinkRepository implements cores.ShortLinkRepository interface with GORM
type GormShortLinkRepository struct {
	db *gorm.DB
}

// NewGormShortLinkRepository creates a new GormShortLinkRepository
func NewGormShortLinkRepository(db *gorm.DB) *GormShortLinkRepository {
	return &GormShortLinkRepository{
		db: db,
	}
}

// Create implements cores.ShortLinkRepository.Create
func (r *GormShortLinkRepository) Create(ctx context.Context, link *cores.ShortLink) error {
	// Convert to GORM model
	model := FromCore(link)

	// Create the record
	result := r.db.WithContext(ctx).Create(model)
	if result.Error != nil {
		return result.Error
	}

	// Update the link ID with the auto-generated ID
	link.ID = model.ID

	return nil
}

// Update implements cores.ShortLinkRepository.Update
func (r *GormShortLinkRepository) Update(ctx context.Context, link *cores.ShortLink) error {
	// Convert to GORM model
	model := FromCore(link)

	// Update the record
	result := r.db.WithContext(ctx).Save(model)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// Updates implements cores.ShortLinkRepository.Updates
func (r *GormShortLinkRepository) Updates(ctx context.Context, links []*cores.ShortLink) error {
	// Begin a transaction
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Defer rollback in case of error
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update each link
	for _, link := range links {
		model := FromCore(link)
		result := tx.Save(model)
		if result.Error != nil {
			tx.Rollback()
			return result.Error
		}
	}

	// Commit the transaction
	return tx.Commit().Error
}

// Delete implements cores.ShortLinkRepository.Delete
func (r *GormShortLinkRepository) Delete(ctx context.Context, id int64) error {
	// Delete the record
	result := r.db.WithContext(ctx).Delete(&ShortLinkModel{}, id)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetByCode implements cores.ShortLinkRepository.GetByCode
func (r *GormShortLinkRepository) GetByCode(ctx context.Context, code string) (*cores.ShortLink, error) {
	// Find the record by code
	var model ShortLinkModel
	result := r.db.WithContext(ctx).Where("code = ?", code).First(&model)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, ErrLinkNotFound
		}
		return nil, result.Error
	}

	// Convert to core model
	return model.ToCore(), nil
}

// GetByID implements cores.ShortLinkRepository.GetByID
func (r *GormShortLinkRepository) GetByID(ctx context.Context, id int64) (*cores.ShortLink, error) {
	// Find the record by ID
	var model ShortLinkModel
	result := r.db.WithContext(ctx).First(&model, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, ErrLinkNotFound
		}
		return nil, result.Error
	}

	// Convert to core model
	return model.ToCore(), nil
}

// FindExpiredActives implements cores.ShortLinkRepository.FindExpiredActives
func (r *GormShortLinkRepository) FindExpiredActives(ctx context.Context, fromID int64, limit int) ([]*cores.ShortLink, error) {
	// Find expired active links
	var models []ShortLinkModel
	result := r.db.WithContext(ctx).
		Where("id > ? AND status = ? AND expire < ?", fromID, cores.LinkStatusActive, time.Now()).
		Order("id ASC").
		Limit(limit).
		Find(&models)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert to core models
	links := make([]*cores.ShortLink, len(models))
	for i, model := range models {
		links[i] = model.ToCore()
	}

	return links, nil
}

// FindExpires implements cores.ShortLinkRepository.FindExpires
func (r *GormShortLinkRepository) FindExpires(ctx context.Context, fromID int64, expiredBefore time.Time, limit int) ([]*cores.ShortLink, error) {
	// Find expired links
	var models []ShortLinkModel
	result := r.db.WithContext(ctx).
		Where("id > ? AND status = ? AND expire < ?", fromID, cores.LinkStatusExpire, expiredBefore).
		Order("id ASC").
		Limit(limit).
		Find(&models)

	if result.Error != nil {
		return nil, result.Error
	}

	// Convert to core models
	links := make([]*cores.ShortLink, len(models))
	for i, model := range models {
		links[i] = model.ToCore()
	}

	return links, nil
}

// GetStartIndex implements cores.ShortLinkRepository.GetStartIndex
func (r *GormShortLinkRepository) GetStartIndex(ctx context.Context, length int) (int64, error) {
	// Find the start index for the given length
	var model StartIndexModel
	result := r.db.WithContext(ctx).Where("length = ?", length).First(&model)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// If not found, return 0
			return 0, nil
		}
		return 0, result.Error
	}

	return model.StartIndex, nil
}

// SaveStartIndex implements cores.ShortLinkRepository.SaveStartIndex
func (r *GormShortLinkRepository) SaveStartIndex(ctx context.Context, length int, index int64) error {
	// Upsert the start index
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "length"}},
		DoUpdates: clause.AssignmentColumns([]string{"start_index", "updated_at"}),
	}).Create(&StartIndexModel{
		Length:     length,
		StartIndex: index,
	})

	if result.Error != nil {
		return result.Error
	}

	return nil
}
