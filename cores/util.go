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

import "strings"

// Base62 character set
const (
	base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	base62base  = int64(len(base62Chars))
)

// ToBase62 converts decimal ID to base62 short code with specified length
func ToBase62(id int64, length int) string {
	var result strings.Builder
	tempID := id

	// Convert to base62
	for tempID > 0 {
		result.WriteByte(base62Chars[tempID%base62base])
		tempID /= base62base
	}

	// Reverse the string
	reversed := reverseString(result.String())

	// Pad with leading zeros
	if len(reversed) < int(length) {
		reversed = strings.Repeat("0", int(length)-len(reversed)) + reversed
	}

	return reversed
}

// Reverse string
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// FromBase62 converts base62 short code to decimal ID
func FromBase62(shortCode string) int64 {
	var id int64

	for _, c := range shortCode {
		pos := strings.IndexRune(base62Chars, c)
		if pos == -1 {
			return -1 // Invalid character
		}
		id = id*base62base + int64(pos)
	}

	return id
}
