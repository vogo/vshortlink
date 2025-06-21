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
	"time"

	"github.com/vogo/vshortlink/cores"
	"gorm.io/gorm"
)

// ShortLinkModel is the GORM model for short links
type ShortLinkModel struct {
	ID        int64            `gorm:"primaryKey;autoIncrement"`
	Length    int              `gorm:"index"`
	Code      string           `gorm:"uniqueIndex;size:32"`
	Link      string           `gorm:"size:2048"`
	Expire    time.Time        `gorm:"index"`
	Status    cores.LinkStatus `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for the ShortLinkModel
func (ShortLinkModel) TableName() string {
	return "short_links"
}

// StartIndexModel is the GORM model for storing start indices
type StartIndexModel struct {
	Length     int   `gorm:"primaryKey"`
	StartIndex int64 `gorm:"column:start_index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TableName returns the table name for the StartIndexModel
func (StartIndexModel) TableName() string {
	return "short_link_indices"
}

// PoolLockModel is the GORM model for pool locks
type PoolLockModel struct {
	Length    int       `gorm:"primaryKey"`
	ExpireAt  time.Time `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName returns the table name for the PoolLockModel
func (PoolLockModel) TableName() string {
	return "short_link_pool_locks"
}

// ToCore converts a ShortLinkModel to a cores.ShortLink
func (m *ShortLinkModel) ToCore() *cores.ShortLink {
	return &cores.ShortLink{
		ID:     m.ID,
		Length: m.Length,
		Code:   m.Code,
		Link:   m.Link,
		Expire: m.Expire,
		Status: m.Status,
	}
}

// FromCore converts a cores.ShortLink to a ShortLinkModel
func FromCore(link *cores.ShortLink) *ShortLinkModel {
	return &ShortLinkModel{
		ID:     link.ID,
		Length: link.Length,
		Code:   link.Code,
		Link:   link.Link,
		Expire: link.Expire,
		Status: link.Status,
	}
}
