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
	"strings"
	"testing"
	"time"
)

// newStatsOnlyService constructs a ShortLinkService with just the fields the
// stats code path exercises. It bypasses NewShortLinkService so the test does
// not need a real Repo/Cache/Pool.
func newStatsOnlyService(m ShortLinkStats, retentionDays int) *ShortLinkService {
	return &ShortLinkService{
		Stats:              m,
		statsLoc:           time.UTC,
		statsRetentionDays: retentionDays,
	}
}

func TestGetStats_Disabled(t *testing.T) {
	svc := &ShortLinkService{statsLoc: time.UTC, statsRetentionDays: 7}

	if _, err := svc.GetStats(context.Background(), "abc", 3); err == nil ||
		!strings.Contains(err.Error(), "stats not enabled") {
		t.Fatalf("GetStats without Stats: want 'stats not enabled', got %v", err)
	}
	if _, err := svc.BatchGetStats(context.Background(), []string{"abc"}, 3); err == nil ||
		!strings.Contains(err.Error(), "stats not enabled") {
		t.Fatalf("BatchGetStats without Stats: want 'stats not enabled', got %v", err)
	}
}

func TestGetStats_InvalidArgs(t *testing.T) {
	svc := newStatsOnlyService(newMockStats(), 7)

	if _, err := svc.GetStats(context.Background(), "", 3); err == nil {
		t.Fatalf("expected error for empty code")
	}
	if _, err := svc.BatchGetStats(context.Background(), nil, 3); err == nil {
		t.Fatalf("expected error for empty codes")
	}
	if _, err := svc.BatchGetStats(context.Background(), []string{}, 3); err == nil {
		t.Fatalf("expected error for empty codes slice")
	}
}

func TestGetStats_DaysCappedAtRetention(t *testing.T) {
	m := newMockStats()
	svc := newStatsOnlyService(m, 3)

	// days > retention → capped to retention.
	if _, err := svc.GetStats(context.Background(), "abc", 999); err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if got := len(m.lastGetDays); got != 3 {
		t.Fatalf("Get days passed to backend=%d want 3", got)
	}

	// days <= 0 → defaulted to retention.
	if _, err := svc.GetStats(context.Background(), "abc", 0); err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if got := len(m.lastGetDays); got != 3 {
		t.Fatalf("days=0 should default to retention=3, got %d", got)
	}

	// Same cap applies to BatchGetStats.
	if _, err := svc.BatchGetStats(context.Background(), []string{"abc"}, 999); err != nil {
		t.Fatalf("BatchGetStats: %v", err)
	}
	if got := len(m.lastBatchGetDays); got != 3 {
		t.Fatalf("BatchGet days passed to backend=%d want 3", got)
	}
}

func TestGetStats_DaysListAscendingIncludesToday(t *testing.T) {
	m := newMockStats()
	svc := newStatsOnlyService(m, 7)

	if _, err := svc.GetStats(context.Background(), "abc", 3); err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	days := m.lastGetDays
	if len(days) != 3 {
		t.Fatalf("len(days)=%d want 3", len(days))
	}
	today := time.Now().In(time.UTC).Format(StatsDayFormat)
	if days[len(days)-1] != today {
		t.Fatalf("last day=%s want today=%s", days[len(days)-1], today)
	}
	// Ascending order check.
	for i := 1; i < len(days); i++ {
		if days[i-1] >= days[i] {
			t.Fatalf("days not ascending: %v", days)
		}
	}
}

func TestRecordHit_NoRecorder_NoPanic(t *testing.T) {
	svc := &ShortLinkService{}
	// Must not panic when stats/recorder are absent.
	svc.RecordHit("anything")
}

func TestService_RecordsThenQueryReturnsCounts(t *testing.T) {
	m := newMockStats()
	svc := &ShortLinkService{
		Stats:              m,
		statsLoc:           time.UTC,
		statsRetentionDays: 7,
		statsFlushInterval: 20 * time.Millisecond,
		statsBufferSize:    1024,
	}
	svc.statsRecorder = newStatsRecorder(m, time.UTC, 1024, 20*time.Millisecond)
	stop := make(chan struct{})
	go svc.statsRecorder.run(stop)
	t.Cleanup(func() {
		close(stop)
		<-svc.statsRecorder.done
	})

	for range 3 {
		svc.RecordHit("abc")
	}
	svc.RecordHit("xyz")

	waitFor(t, 500*time.Millisecond, func() bool { return m.totalCalls() > 0 })

	today := time.Now().In(time.UTC).Format(StatsDayFormat)

	res, err := svc.GetStats(context.Background(), "abc", 1)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if got := res[today]; got != 3 {
		t.Fatalf("abc today=%d want 3", got)
	}

	batch, err := svc.BatchGetStats(context.Background(), []string{"abc", "xyz", "missing"}, 1)
	if err != nil {
		t.Fatalf("BatchGetStats: %v", err)
	}
	if got := batch["abc"][today]; got != 3 {
		t.Fatalf("batch abc=%d want 3", got)
	}
	if got := batch["xyz"][today]; got != 1 {
		t.Fatalf("batch xyz=%d want 1", got)
	}
	if _, exists := batch["missing"]; exists {
		t.Fatalf("batch should omit codes with no hits, got %v", batch)
	}
}
