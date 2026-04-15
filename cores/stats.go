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

import "context"

// StatsDayFormat is the canonical day-key layout used by the stats subsystem.
// Recorders format the current day with this layout; backends treat the value
// as an opaque string partition key.
const StatsDayFormat = "20060102"

// ShortLinkStats persists per-code daily hit counters. Implementations are
// expected to accept a day string (see StatsDayFormat) as an opaque partition
// key and to enforce their own retention (e.g. Redis TTL).
type ShortLinkStats interface {
	// IncrBatch merges the given code->delta map into the counters of `day`.
	// Called from the recorder flush goroutine, not from the hot redirect path.
	IncrBatch(ctx context.Context, day string, deltas map[string]int64) error

	// Get returns the counters of one code for the given days. Missing days
	// are omitted from the returned map.
	Get(ctx context.Context, code string, days []string) (map[string]int64, error)

	// BatchGet returns counters for many codes over many days.
	// Outer key: code; inner key: day. Codes/days with no hits are omitted.
	BatchGet(ctx context.Context, codes []string, days []string) (map[string]map[string]int64, error)

	Close(ctx context.Context) error
}
