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
	"testing"
)

func TestConvertToBase62(t *testing.T) {
	// Test case structure
	tests := []struct {
		name     string
		id       int64
		length   int
		expected string
	}{
		// Normal case tests
		{"Normal 4-digit code", 2382328, 4, "9ZKE"},     // Minimum 4-digit ID
		{"Normal 5-digit code", 147763336, 5, "9ZZZC"},  // Minimum 5-digit ID
		{"Normal 6-digit code", 916132832, 6, "100000"}, // Minimum 6-digit ID
		{"Random ID conversion", 12345678, 4, "PNFQ"},
		{"Large value conversion", 9876543210, 6, "aMoY42"},

		// Boundary case tests - Basic values
		{"ID is 0", 0, 4, "0000"},   // Should return all zeros when ID is 0
		{"ID is 1", 1, 4, "0001"},   // Should return 0001 with specified length when ID is 1
		{"ID is 61", 61, 4, "000Z"}, // Should return 000Z with specified length when ID is 61
		{"ID is 62", 62, 4, "0010"}, // Should return 0010 with specified length when ID is 62

		// Boundary case tests - 4-digit short code
		{"4-digit code minimum", 0, 4, "0000"},           // 4-digit code minimum value
		{"4-digit code maximum", 14776335, 4, "ZZZZ"},    // 4-digit code maximum value
		{"4-digit code maximum+1", 14776336, 4, "10000"}, // 4-digit code maximum value+1, becomes 5-digit

		// Boundary case tests - 5-digit short code
		{"5-digit code minimum", 0, 5, "00000"},            // 5-digit code minimum value
		{"5-digit code maximum", 916132831, 5, "ZZZZZ"},    // 5-digit code maximum value
		{"5-digit code maximum+1", 916132832, 5, "100000"}, // 5-digit code maximum value+1, becomes 6-digit

		// Boundary case tests - 6-digit short code
		{"6-digit code minimum", 0, 6, "000000"},              // 6-digit code minimum value
		{"6-digit code maximum", 56800235583, 6, "ZZZZZZ"},    // 6-digit code maximum value
		{"6-digit code maximum+1", 56800235584, 6, "1000000"}, // 6-digit code maximum value+1, becomes 7-digit

		// Length padding tests
		{"Padding-shorter than specified", 123, 4, "001Z"},    // Actual length less than specified, needs padding
		{"Padding-equal to specified", 14776335, 4, "ZZZZ"},   // Actual length equals specified length
		{"Padding-exceeds specified", 916132832, 4, "100000"}, // Actual length exceeds specified length, keep original length
	}

	// Execute test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToBase62(tt.id, tt.length)
			if got != tt.expected {
				t.Errorf("ConvertToBase62(%d, %d) = %s, expected %s", tt.id, tt.length, got, tt.expected)
			}
		})
	}
}

func TestReverseString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Single character", "a", "a"},
		{"Normal string", "abcdef", "fedcba"},
		{"Numeric string", "123456", "654321"},
		{"Mixed string", "a1b2c3", "3c2b1a"},
		{"Chinese string", "你好世界", "界世好你"}, // Test Unicode characters
		{"Special characters", "!@#$%^", "^%$#@!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reverseString(tt.input)
			if got != tt.expected {
				t.Errorf("reverseString(%s) = %s, expected %s", tt.input, got, tt.expected)
			}
		})
	}
}

func TestConvertFromBase62(t *testing.T) {
	tests := []struct {
		name      string
		shortCode string
		expected  int64
	}{
		// Normal case tests
		{"Normal 4-digit code", "9ZKE", 2382328},     // Minimum 4-digit ID
		{"Normal 5-digit code", "9ZZZC", 147763336},  // Minimum 5-digit ID
		{"Normal 6-digit code", "100000", 916132832}, // Minimum 6-digit ID
		{"Random short code", "PNFQ", 12345678},
		{"Large value short code", "aMoY42", 9876543210},

		// Boundary case tests - Basic values
		{"All zeros short code", "0000", 0},
		{"Minimum non-zero short code", "0001", 1},
		{"Maximum single character short code", "Z", 61},
		{"Carry short code", "10", 62},

		// Boundary case tests - 4-digit short code
		{"4-digit code minimum", "0000", 0},                     // 4-digit code minimum value
		{"4-digit code maximum", "ZZZZ", 14776335},              // 4-digit code maximum value
		{"4-digit code overflow to 5-digit", "10000", 14776336}, // 4-digit code overflow to 5-digit

		// Boundary case tests - 5-digit short code
		{"5-digit code minimum", "00000", 0},                      // 5-digit code minimum value
		{"5-digit code maximum", "ZZZZZ", 916132831},              // 5-digit code maximum value
		{"5-digit code overflow to 6-digit", "100000", 916132832}, // 5-digit code overflow to 6-digit

		// Boundary case tests - 6-digit short code
		{"6-digit code minimum", "000000", 0},                        // 6-digit code minimum value
		{"6-digit code maximum", "ZZZZZZ", 56800235583},              // 6-digit code maximum value
		{"6-digit code overflow to 7-digit", "1000000", 56800235584}, // 6-digit code overflow to 7-digit

		// Special case tests
		{"Empty string", "", 0},
		{"Leading zeros", "0001", 1}, // Leading zeros don't affect the result
		// Note: Converting the entire character set would cause integer overflow, actual results depend on Go's int64 overflow behavior
		{"Partial character set", "0123456789ab", 867042935339397963}, // Using partial character set to avoid overflow
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromBase62(tt.shortCode)
			if got != tt.expected {
				t.Errorf("ConvertFromBase62(%s) = %d, expected %d", tt.shortCode, got, tt.expected)
			}
		})
	}
}

func TestConvertFromBase62_InvalidChars(t *testing.T) {
	tests := []struct {
		name      string
		shortCode string
	}{
		{"Invalid character - Space", "123 456"},
		{"Invalid character - Special symbol", "123#456"},
		{"Invalid character - Chinese", "123你好"},
		{"Invalid character - Japanese", "123あいう"},
		{"Invalid character - Emoji", "123😊"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromBase62(tt.shortCode)
			if got != -1 {
				t.Errorf("ConvertFromBase62(%s) = %d, expected -1 (invalid character)", tt.shortCode, got)
			}
		})
	}
}

func TestConvertToBase62AndBack(t *testing.T) {
	// Test if conversion back and forth is consistent
	tests := []struct {
		name   string
		id     int64
		length int
	}{
		// Regular value tests
		{"Small value", 123, 4},
		{"Medium value", 12345678, 4},
		{"Large value", 9876543210, 6},
		{"4-digit minimum ID", 2382328, 4},
		{"5-digit minimum ID", 147763336, 5},
		{"6-digit minimum ID", 916132832, 6},

		// Boundary value tests - Basic values
		{"ID is 0", 0, 4},
		{"ID is 1", 1, 4},
		{"ID is 61", 61, 4},
		{"ID is 62", 62, 4},

		// Boundary value tests - 4-digit short code
		{"4-digit code minimum", 0, 4},
		{"4-digit code maximum", 14776335, 4},
		{"4-digit code maximum+1", 14776336, 4},

		// Boundary value tests - 5-digit short code
		{"5-digit code minimum", 0, 5},
		{"5-digit code maximum", 916132831, 5},
		{"5-digit code maximum+1", 916132832, 5},

		// Boundary value tests - 6-digit short code
		{"6-digit code minimum", 0, 6},
		{"6-digit code maximum", 56800235583, 6},
		{"6-digit code maximum+1", 56800235584, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shortCode := ToBase62(tt.id, tt.length)
			gotID := FromBase62(shortCode)
			if gotID != tt.id {
				t.Errorf("Conversion inconsistency: %d -> %s -> %d", tt.id, shortCode, gotID)
			}
		})
	}
}

func BenchmarkConvertToBase62(b *testing.B) {
	// Performance test
	for i := 0; i < b.N; i++ {
		ToBase62(12345678, 4)
	}
}

func BenchmarkConvertFromBase62(b *testing.B) {
	// Performance test
	for i := 0; i < b.N; i++ {
		FromBase62("4fI2")
	}
}

func BenchmarkReverseString(b *testing.B) {
	// Performance test
	for i := 0; i < b.N; i++ {
		reverseString("abcdefghijklmnopqrstuvwxyz")
	}
}
