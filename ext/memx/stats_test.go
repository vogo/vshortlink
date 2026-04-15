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

package memx

import (
	"context"
	"testing"
)

func TestMemStats_IncrAndGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryShortLinkStats()

	if err := s.IncrBatch(ctx, "20260415", map[string]int64{"a": 3, "b": 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.IncrBatch(ctx, "20260415", map[string]int64{"a": 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.IncrBatch(ctx, "20260414", map[string]int64{"a": 4}); err != nil {
		t.Fatal(err)
	}

	out, err := s.Get(ctx, "a", []string{"20260415", "20260414", "20260413"})
	if err != nil {
		t.Fatal(err)
	}
	if out["20260415"] != 5 {
		t.Errorf("20260415=%d want 5", out["20260415"])
	}
	if out["20260414"] != 4 {
		t.Errorf("20260414=%d want 4", out["20260414"])
	}
	if _, ok := out["20260413"]; ok {
		t.Errorf("20260413 should be absent, got %v", out)
	}
}

func TestMemStats_BatchGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryShortLinkStats()
	_ = s.IncrBatch(ctx, "20260415", map[string]int64{"a": 1, "b": 2})
	_ = s.IncrBatch(ctx, "20260414", map[string]int64{"a": 3})

	out, err := s.BatchGet(ctx, []string{"a", "b", "c"}, []string{"20260415", "20260414"})
	if err != nil {
		t.Fatal(err)
	}
	if out["a"]["20260415"] != 1 {
		t.Errorf("a/15=%d want 1", out["a"]["20260415"])
	}
	if out["a"]["20260414"] != 3 {
		t.Errorf("a/14=%d want 3", out["a"]["20260414"])
	}
	if out["b"]["20260415"] != 2 {
		t.Errorf("b/15=%d want 2", out["b"]["20260415"])
	}
	if _, ok := out["b"]["20260414"]; ok {
		t.Errorf("b/14 should be absent, got %v", out)
	}
	if _, ok := out["c"]; ok {
		t.Errorf("c should be absent, got %v", out)
	}
}

func TestMemStats_EmptyInputs(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryShortLinkStats()

	if err := s.IncrBatch(ctx, "20260415", nil); err != nil {
		t.Fatal(err)
	}
	out, _ := s.Get(ctx, "a", []string{"20260415"})
	if len(out) != 0 {
		t.Errorf("empty incr then get want empty, got %v", out)
	}

	out2, _ := s.BatchGet(ctx, nil, []string{"20260415"})
	if len(out2) != 0 {
		t.Errorf("empty codes want empty, got %v", out2)
	}

	out3, _ := s.BatchGet(ctx, []string{"a"}, nil)
	if len(out3) != 0 {
		t.Errorf("empty days want empty, got %v", out3)
	}
}

func TestMemStats_Close(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryShortLinkStats()
	_ = s.IncrBatch(ctx, "20260415", map[string]int64{"a": 1})

	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}

	out, _ := s.Get(ctx, "a", []string{"20260415"})
	if len(out) != 0 {
		t.Errorf("Close should clear data, got %v", out)
	}
}
