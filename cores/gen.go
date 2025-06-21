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
	"fmt"
	"strings"
)

type ShortCodeGenerator struct {
	length    int
	step      int64
	batchSize int
}

func NewShortCodeGenerator(length int, batchSize int) *ShortCodeGenerator {
	return &ShortCodeGenerator{
		length:    length,
		batchSize: batchSize,
		step:      FromBase62(strings.Repeat("Z", length)) / int64(batchSize),
	}
}

// GenerateBatchNumbers generate batch short code numbers
func (g *ShortCodeGenerator) GenerateBatchNumbers(length int, startIndex int64) ([]int64, error) {
	if startIndex > g.step {
		return nil, fmt.Errorf("length %d short code pool is exhausted, please use longer short code", g.length)
	}

	batchNumbers := make([]int64, g.batchSize)
	for i := 0; i < g.batchSize; i++ {
		batchNumbers[i] = startIndex
		startIndex += g.step
	}
	return batchNumbers, nil
}
