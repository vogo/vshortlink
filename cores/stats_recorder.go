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
	"sync/atomic"
	"time"

	"github.com/vogo/vogo/vlog"
)

// statsFlushTimeout bounds each backend IncrBatch call so a stalled backend
// cannot pile up flushes and leak goroutines at shutdown.
const statsFlushTimeout = 5 * time.Second

// statsRecorder buffers hit events off the redirect path and flushes them in
// merged batches to the backend. It drops events on a full buffer rather than
// block the redirect — accepted trade-off: precision vs hot-path latency.
type statsRecorder struct {
	stats ShortLinkStats
	loc   *time.Location
	ch    chan string
	flush time.Duration

	drops atomic.Int64
	done  chan struct{}
}

func newStatsRecorder(stats ShortLinkStats, loc *time.Location, bufferSize int, flush time.Duration) *statsRecorder {
	return &statsRecorder{
		stats: stats,
		loc:   loc,
		ch:    make(chan string, bufferSize),
		flush: flush,
		done:  make(chan struct{}),
	}
}

// Record is called on every successful redirect. Non-blocking by design.
func (r *statsRecorder) Record(code string) {
	select {
	case r.ch <- code:
	default:
		r.drops.Add(1)
	}
}

// run owns the merge map — no other goroutine touches it, so no locking.
func (r *statsRecorder) run(stop <-chan struct{}) {
	defer close(r.done)

	merge := make(map[string]int64)
	ticker := time.NewTicker(r.flush)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			r.drain(merge)
			r.flushNow(merge)
			return
		case code := <-r.ch:
			merge[code]++
		case <-ticker.C:
			if len(merge) > 0 {
				r.flushNow(merge)
				merge = make(map[string]int64)
			}
		}
	}
}

func (r *statsRecorder) drain(merge map[string]int64) {
	for {
		select {
		case code := <-r.ch:
			merge[code]++
		default:
			return
		}
	}
}

func (r *statsRecorder) flushNow(merge map[string]int64) {
	if len(merge) == 0 {
		return
	}
	day := time.Now().In(r.loc).Format(StatsDayFormat)
	ctx, cancel := context.WithTimeout(context.Background(), statsFlushTimeout)
	defer cancel()
	if err := r.stats.IncrBatch(ctx, day, merge); err != nil {
		vlog.Errorf("flush stats failed, day=%s size=%d err=%v", day, len(merge), err)
	}
	if dropped := r.drops.Swap(0); dropped > 0 {
		vlog.Warnf("stats recorder dropped %d records (buffer full)", dropped)
	}
}

// dayList returns day strings from today back to today-(days-1), sorted ascending.
func dayList(now time.Time, loc *time.Location, days int) []string {
	if days <= 0 {
		return nil
	}
	out := make([]string, days)
	base := now.In(loc)
	for i := range days {
		out[days-1-i] = base.AddDate(0, 0, -i).Format(StatsDayFormat)
	}
	return out
}
