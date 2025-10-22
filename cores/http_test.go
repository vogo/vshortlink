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
	"encoding/json"
	"testing"
	"time"
)

func TestEditLinkEnocde(t *testing.T) {
	editReq := EditLinkRequest{
		Link:   "https://www.baidu.com",
		Length: 6,
		Expire: time.Now().Add(time.Hour * 24 * 365),
	}

	jsonStr, err := json.Marshal(editReq)
	if err != nil {
		t.Fatalf("failed to marshal edit request: %v", err)
	}

	t.Logf("edit request: %s", jsonStr)
}

func TestEditLinkDeocde(t *testing.T) {
	str := `{"link":"https://www.baidu.com","length":6,"expire":"2026-08-29T15:58:17+08:00"}`

	var editReq EditLinkRequest
	err := json.Unmarshal([]byte(str), &editReq)
	if err != nil {
		t.Fatalf("failed to unmarshal edit request: %v", err)
	}

	t.Logf("edit request: %v", editReq)
}
