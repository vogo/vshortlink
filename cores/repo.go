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

package cores

import (
	"context"
	"time"
)

type ShortLinkRepository interface {
	Create(ctx context.Context, link *ShortLink) error
	Update(ctx context.Context, link *ShortLink) error
	Updates(ctx context.Context, links []*ShortLink) error
	Delete(ctx context.Context, id int64) error
	DeleteByCode(ctx context.Context, code string) error
	GetByCode(ctx context.Context, code string) (*ShortLink, error)
	GetByID(ctx context.Context, id int64) (*ShortLink, error)
	FindExpiredActives(ctx context.Context, fromID int64, limit int) ([]*ShortLink, error)
	FindExpires(ctx context.Context, fromID int64, expiredBefore time.Time, limit int) ([]*ShortLink, error)
	List(ctx context.Context, length int, statuses []LinkStatus, limit int, fromID int64, ascSort bool) ([]*ShortLink, error)
	GetStartIndex(ctx context.Context, length int) (int64, error)
	SaveStartIndex(ctx context.Context, length int, index int64) error
}
