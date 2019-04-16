// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package coldata

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/exec/types"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
)

func TestMemColumnSlice(t *testing.T) {
	defer leaktest.AfterTest(t)()

	rng, _ := randutil.NewPseudoRand()

	c := NewMemColumn(types.Int64, BatchSize)

	ints := c.Int64()
	for i := uint16(0); i < BatchSize; i++ {
		ints[i] = int64(i)
		if i%2 == 0 {
			// Set every other value to null.
			c.SetNull(i)
		}
	}

	startSlice := uint16(1)
	endSlice := uint16(0)
	for startSlice > endSlice {
		startSlice = uint16(rng.Intn(BatchSize))
		endSlice = uint16(1 + rng.Intn(BatchSize))
	}

	slice := c.Slice(types.Int64, uint64(startSlice), uint64(endSlice))
	sliceInts := slice.Int64()
	// Verify that every other value is null.
	for i, j := startSlice, uint16(0); i < endSlice; i, j = i+1, j+1 {
		if i%2 == 0 {
			if !slice.NullAt(j) {
				t.Fatalf("expected null at %d (original index: %d)", j, i)
			}
			continue
		}
		if ints[i] != sliceInts[j] {
			t.Fatalf("unexected value at index %d (original index: %d): expected %d got %d", j, i, ints[i], sliceInts[j])
		}
	}
}
