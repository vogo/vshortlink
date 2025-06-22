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
	"net/http"
	"time"

	"github.com/vogo/vogo/vencoding/vjson"
	"github.com/vogo/vogo/vnet/vhttp/vhttpresp"
)

func (s *ShortLinkService) HttpHandle(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Path[1:]
	if code == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if code == "create_link" {
		s.HandleCreate(w, r)
		return
	}

	// check whether the code match the format
	if len(code) > s.maxCodeLength {
		http.NotFound(w, r)
		return
	}

	link, ok := s.Cache.Get(r.Context(), len(code), code)
	if !ok || link == "" {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, link, http.StatusFound)
}

type CreateLinkRequest struct {
	Link   string    `json:"link"`
	Length int       `json:"length"`
	Expire time.Time `json:"expire"`
}

func (s *ShortLinkService) HandleCreate(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if s.authToken != "" && token != s.authToken {
		vhttpresp.BadMsg(w, r, "unauthorized")
		return
	}

	var req CreateLinkRequest
	err := vjson.UnmarshalStream(r.Body, &req)
	if err != nil {
		vhttpresp.BadError(w, r, err)
		return
	}

	if req.Link == "" {
		vhttpresp.BadMsg(w, r, "link is empty")
		return
	}

	if req.Length == 0 || req.Length > s.maxCodeLength {
		vhttpresp.BadMsg(w, r, "invalid code length")
		return
	}

	if req.Expire.IsZero() || req.Expire.Before(time.Now()) {
		req.Expire = time.Now().Add(24 * time.Hour)
	}

	shortLink, err := s.Create(r.Context(), req.Link, req.Length, req.Expire)
	if err != nil {
		vhttpresp.BadError(w, r, err)
		return
	}

	vhttpresp.Success(w, r, shortLink)
}
