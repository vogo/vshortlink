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

import "time"

type LinkStatus int

const (
	LinkStatusActive   LinkStatus = 1 // active
	LinkStatusExpired  LinkStatus = 2 // expired
	LinkStatusRecycled LinkStatus = 3 // recycled
)

type ShortLink struct {
	ID         int64      `json:"id" comment:"ID"`
	Length     int        `json:"length" comment:"short code length"`
	Code       string     `json:"code" comment:"short code"`
	Title      string     `json:"title" comment:"link title"`
	Link       string     `json:"link" comment:"original link"`
	Expire     time.Time  `json:"expire" comment:"expire time"`
	Status     LinkStatus `json:"status" comment:"status"`
	CreateTime time.Time  `json:"create_time" comment:"create time"`
	ModifyTime time.Time  `json:"modify_time" comment:"modify time"`
}

func (l *ShortLink) IsActive() bool {
	return l.Status == LinkStatusActive
}

func (l *ShortLink) IsExpired() bool {
	return l.Status == LinkStatusExpired
}

func (l *ShortLink) IsRecycle() bool {
	return l.Status == LinkStatusRecycled
}
