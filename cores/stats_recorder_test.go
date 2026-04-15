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
	"maps"
	"sync"
	"testing"
	"time"
)

// mockStats is an in-package test double for ShortLinkStats. Shared by
// stats_recorder_test.go and service_stats_test.go.
type mockStats struct {
	mu               sync.Mutex
	calls            []mockStatsCall
	data             map[string]map[string]int64 // day -> code -> total
	lastGetDays      []string
	lastBatchGetDays []string
}

type mockStatsCall struct {
	day    string
	deltas map[string]int64
}

func newMockStats() *mockStats {
	return &mockStats{data: map[string]map[string]int64{}}
}

func (m *mockStats) IncrBatch(_ context.Context, day string, deltas map[string]int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make(map[string]int64, len(deltas))
	maps.Copy(cp, deltas)
	m.calls = append(m.calls, mockStatsCall{day: day, deltas: cp})
	d, ok := m.data[day]
	if !ok {
		d = map[string]int64{}
		m.data[day] = d
	}
	for k, v := range deltas {
		d[k] += v
	}
	return nil
}

func (m *mockStats) Get(_ context.Context, code string, days []string) (map[string]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastGetDays = append([]string(nil), days...)
	out := map[string]int64{}
	for _, day := range days {
		if d, ok := m.data[day]; ok {
			if v, ok := d[code]; ok && v != 0 {
				out[day] = v
			}
		}
	}
	return out, nil
}

func (m *mockStats) BatchGet(_ context.Context, codes []string, days []string) (map[string]map[string]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastBatchGetDays = append([]string(nil), days...)
	out := map[string]map[string]int64{}
	for _, day := range days {
		d, ok := m.data[day]
		if !ok {
			continue
		}
		for _, code := range codes {
			v, ok := d[code]
			if !ok || v == 0 {
				continue
			}
			if _, exists := out[code]; !exists {
				out[code] = map[string]int64{}
			}
			out[code][day] = v
		}
	}
	return out, nil
}

func (m *mockStats) Close(_ context.Context) error { return nil }

func (m *mockStats) totalCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockStats) total(day, code string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[day][code]
}

func TestDayList(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	got := dayList(now, time.UTC, 3)
	want := []string{"20260413", "20260414", "20260415"}
	if !equalStrings(got, want) {
		t.Fatalf("dayList(3)=%v want %v", got, want)
	}

	if out := dayList(now, time.UTC, 0); out != nil {
		t.Fatalf("days=0 want nil, got %v", out)
	}
	if out := dayList(now, time.UTC, -1); out != nil {
		t.Fatalf("days<0 want nil, got %v", out)
	}

	// Timezone changes which day "today" is: 20:00 UTC == 04:00 next day in UTC+8.
	night := time.Date(2026, 4, 15, 20, 0, 0, 0, time.UTC)
	sh, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skip("timezone db unavailable")
	}
	if utc := dayList(night, time.UTC, 1); utc[0] != "20260415" {
		t.Fatalf("utc today=%s want 20260415", utc[0])
	}
	if cst := dayList(night, sh, 1); cst[0] != "20260416" {
		t.Fatalf("shanghai today=%s want 20260416", cst[0])
	}
}

func TestStatsRecorder_FlushesMergedBatch(t *testing.T) {
	m := newMockStats()
	rec := newStatsRecorder(m, time.UTC, 1024, 20*time.Millisecond)
	stop := make(chan struct{})
	go rec.run(stop)
	t.Cleanup(func() {
		close(stop)
		<-rec.done
	})

	for range 5 {
		rec.Record("abc")
	}
	for range 3 {
		rec.Record("xyz")
	}

	waitFor(t, 500*time.Millisecond, func() bool { return m.totalCalls() > 0 })

	day := time.Now().In(time.UTC).Format(StatsDayFormat)
	if got := m.total(day, "abc"); got != 5 {
		t.Fatalf("abc total=%d want 5", got)
	}
	if got := m.total(day, "xyz"); got != 3 {
		t.Fatalf("xyz total=%d want 3", got)
	}
}

func TestStatsRecorder_DropsOnFullBuffer(t *testing.T) {
	m := newMockStats()
	// Never start run(): the channel fills and stays full, so Record takes the
	// default branch and increments drops deterministically.
	rec := newStatsRecorder(m, time.UTC, 2, time.Hour)

	const sent = 100
	for range sent {
		rec.Record("z")
	}

	// Channel holds 2, remaining (sent-2) should be dropped.
	if got := rec.drops.Load(); got != int64(sent-2) {
		t.Fatalf("drops=%d want %d", got, sent-2)
	}
}

func TestStatsRecorder_StopDrainsAndFlushes(t *testing.T) {
	m := newMockStats()
	// Long flush so the periodic tick never fires; Stop must still flush.
	rec := newStatsRecorder(m, time.UTC, 1024, time.Hour)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		rec.run(stop)
		close(done)
	}()

	for range 7 {
		rec.Record("k")
	}

	close(stop)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("recorder did not stop within 1s")
	}

	day := time.Now().In(time.UTC).Format(StatsDayFormat)
	if got := m.total(day, "k"); got != 7 {
		t.Fatalf("k total after stop=%d want 7", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}
