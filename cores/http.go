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
	"net/http"
	"strings"
	"time"

	"github.com/vogo/vogo/vencoding/vjson"
	"github.com/vogo/vogo/vlog"
	"github.com/vogo/vogo/vnet/vhttp/vhttpquery"
	"github.com/vogo/vogo/vnet/vhttp/vhttpresp"
)

const (
	ManagementCodePrefix = "__"
)

func (s *ShortLinkService) HttpHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	code := r.URL.Path[1:]
	if code == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if strings.HasPrefix(code, ManagementCodePrefix) {
		s.HttpHandleManagement(w, r, code[len(ManagementCodePrefix):])
		return
	}

	// check whether the code match the format
	if len(code) > s.maxCodeLength {
		http.NotFound(w, r)
		return
	}

	link, ok := s.GetMemLRUCache(code)
	if !ok {
		link, ok = s.Cache.Get(r.Context(), len(code), code)
		if !ok || link == "" {
			http.NotFound(w, r)
			return
		}

		s.AddMemLRUCache(code, link)
	}

	http.Redirect(w, r, link, http.StatusFound)
}

func (s *ShortLinkService) HttpHandleManagement(w http.ResponseWriter, r *http.Request, managementOp string) {
	token := r.Header.Get("Authorization")
	if s.authToken != "" && token != s.authToken {
		vhttpresp.BadMsg(w, r, "unauthorized")
		return
	}

	switch managementOp {
	case "get":
		s.httpHandleGet(w, r)
	case "create":
		s.httpHandleEdit(w, r, s.httpHandleCreate)
	case "update":
		s.httpHandleEdit(w, r, s.httpHandleUpdate)
	case "remove":
		s.httpHandleEdit(w, r, s.httpHandleRemove)
	case "list":
		s.httpHandleList(w, r)
	default:
		vhttpresp.BadMsg(w, r, "invalid op")
		return
	}
}

func (s *ShortLinkService) httpHandleGet(w http.ResponseWriter, r *http.Request) {
	code, ok := vhttpquery.String(r, "code")
	if !ok || code == "" {
		vhttpresp.BadMsg(w, r, "code is empty")
		return
	}

	link, err := s.Repo.GetByCode(r.Context(), code)
	if err != nil {
		vhttpresp.BadError(w, r, err)
		return
	}

	vhttpresp.Success(w, r, link)
}

type EditLinkRequest struct {
	Code   string    `json:"code"`
	Title  string    `json:"title"`
	Link   string    `json:"link"`
	Length int       `json:"length"`
	Expire time.Time `json:"expire"`
}

func (s *ShortLinkService) httpHandleEdit(w http.ResponseWriter, r *http.Request, editHandler func(w http.ResponseWriter, r *http.Request, req *EditLinkRequest)) {
	var req EditLinkRequest
	err := vjson.UnmarshalStream(r.Body, &req)
	if err != nil {
		vhttpresp.BadError(w, r, err)
		return
	}

	editHandler(w, r, &req)
}

func (s *ShortLinkService) httpHandleCreate(w http.ResponseWriter, r *http.Request, req *EditLinkRequest) {
	if req.Link == "" {
		vhttpresp.BadMsg(w, r, "link is empty")
		return
	}
	if req.Title == "" {
		vhttpresp.BadMsg(w, r, "title is empty")
		return
	}

	if req.Length == 0 || req.Length > s.maxCodeLength {
		vhttpresp.BadMsg(w, r, "invalid code length")
		return
	}

	if req.Length <= s.manualCodeLength {
		if req.Code == "" {
			vhttpresp.BadMsg(w, r, "code is empty")
			return
		}

		exists, err := s.Repo.GetByCode(r.Context(), req.Code)
		if err != nil {
			vhttpresp.BadError(w, r, err)
			return
		}
		if exists != nil && exists.Status != LinkStatusRecycled {
			vhttpresp.BadMsg(w, r, "code already exists")
			return
		}
	}

	if req.Expire.IsZero() || req.Expire.Before(time.Now()) {
		req.Expire = time.Now().Add(24 * time.Hour)
	}

	var shortLink *ShortLink
	var err error
	if req.Length <= s.manualCodeLength {
		shortLink, err = s.Add(context.Background(), req.Code, req.Title, req.Link, req.Expire)
	} else {
		shortLink, err = s.Create(context.Background(), req.Title, req.Link, req.Length, req.Expire)
	}

	if err != nil {
		vhttpresp.BadError(w, r, err)
		return
	}

	vlog.Infof("create short link, code:%s, title:%s, link:%s, expire:%s",
		shortLink.Code, shortLink.Title, shortLink.Link, shortLink.Expire)

	vhttpresp.Success(w, r, shortLink)
}

func (s *ShortLinkService) httpHandleUpdate(w http.ResponseWriter, r *http.Request, req *EditLinkRequest) {
	if req.Code == "" || req.Title == "" || req.Link == "" || req.Expire.IsZero() {
		vhttpresp.BadMsg(w, r, "code, title, link, expire is empty")
		return
	}

	err := s.Update(context.Background(), req.Code, req.Title, req.Link, req.Expire)
	if err != nil {
		vhttpresp.BadError(w, r, err)
		return
	}

	vlog.Infof("update short link, code:%s, title:%s, link:%s, expire:%s",
		req.Code, req.Title, req.Link, req.Expire)

	vhttpresp.Success(w, r, nil)
}

func (s *ShortLinkService) httpHandleRemove(w http.ResponseWriter, r *http.Request, req *EditLinkRequest) {
	if req.Code == "" {
		vhttpresp.BadMsg(w, r, "code is empty")
		return
	}

	err := s.Remove(context.Background(), req.Code)
	if err != nil {
		vhttpresp.BadError(w, r, err)
		return
	}

	vlog.Infof("remove short link, code:%s", req.Code)

	vhttpresp.Success(w, r, nil)
}

func (s *ShortLinkService) httpHandleList(w http.ResponseWriter, r *http.Request) {
	length, ok := vhttpquery.Int(r, "length")
	if !ok {
		vhttpresp.BadMsg(w, r, "invalid length")
		return
	}

	from, _ := vhttpquery.Int64(r, "from")

	status, _ := vhttpquery.Int(r, "status")
	statusList := []LinkStatus{}
	if status > 0 {
		statusList = []LinkStatus{LinkStatus(status)}
	}

	limit, _ := vhttpquery.Int(r, "limit")
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	asc, _ := vhttpquery.Bool(r, "asc")

	shortLinks, err := s.Repo.List(context.Background(), length, statusList, limit, from, asc)
	if err != nil {
		vhttpresp.BadError(w, r, err)
		return
	}

	vhttpresp.Success(w, r, shortLinks)
}
