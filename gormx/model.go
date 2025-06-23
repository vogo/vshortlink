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
)

var (
	shortLinkTableName  = "short_links"
	startIndexTableName = "short_link_indices"
)

func SetShortLinkTableName(name string) {
	shortLinkTableName = name
}

func SetStartIndexTableName(name string) {
	startIndexTableName = name
}

// ShortLinkModel is the GORM model for short links
type ShortLinkModel struct {
	ID         int64            `json:"id" gorm:"primaryKey;autoIncrement" comment:"ID"`
	Length     int              `json:"length" gorm:"index" comment:"short code length"`
	Code       string           `json:"code" gorm:"uniqueIndex;size:32" comment:"short code"`
	Link       string           `json:"link" gorm:"size:2048" comment:"original link"`
	Expire     time.Time        `json:"expire" gorm:"index" comment:"expire time"`
	Status     cores.LinkStatus `json:"status" gorm:"index" comment:"status"`
	CreateTime time.Time        `json:"create_time" gorm:"column:create_time" comment:"create time"`
	ModifyTime time.Time        `json:"modify_time" gorm:"column:modify_time" comment:"modify time"`
}

// TableName returns the table name for the ShortLinkModel
func (ShortLinkModel) TableName() string {
	return shortLinkTableName
}

// StartIndexModel is the GORM model for storing start indices
type StartIndexModel struct {
	Length     int       `json:"length" gorm:"primaryKey" comment:"short code length"`
	StartIndex int64     `json:"start_index" gorm:"column:start_index" comment:"start index"`
	CreateTime time.Time `json:"create_time" gorm:"column:create_time" comment:"create time"`
	ModifyTime time.Time `json:"modify_time" gorm:"column:modify_time" comment:"modify time"`
}

// TableName returns the table name for the StartIndexModel
func (StartIndexModel) TableName() string {
	return startIndexTableName
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
