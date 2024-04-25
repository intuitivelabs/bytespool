// Copyright 2021 Intuitive Labs GmbH. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE.txt file in the root of the source
// tree.

package bytespool

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

var seed int64

func init() {
}

func TestMain(m *testing.M) {
	seed = time.Now().UnixNano()
	flag.Int64Var(&seed, "seed", seed, "random seed")
	flag.Parse()
	rand.Seed(seed)
	fmt.Printf("using random seed 0x%x\n", seed)
	res := m.Run()
	os.Exit(res)
}

func TestPoolIdx(t *testing.T) {

	for i := 0; i < 100000; i++ {
		sz := int(rand.Int63())
		roundTo := 1
		if i != 0 {
			// test once with roundTo 1
			roundTo = int(rand.Int63n(1025)) + 1
		}
		minSz := int(rand.Int63n(1025))

		idx := szPoolIdx(sz, roundTo, minSz)
		rmin, rmax := idxSzRange(idx, roundTo, minSz)

		if sz < rmin || sz > rmax {
			t.Fatalf("failed for sz %d => idx %d [%d, %d]"+
				" (roundTo %d, min %d)\n",
				sz, idx, rmin, rmax, roundTo, minSz)
		}
		idxMax := szPoolIdx(rmax, roundTo, minSz)
		idxMin := szPoolIdx(rmin, roundTo, minSz)
		if idxMax != idx || idxMin != idx {
			rmin1, rmin2 := idxSzRange(idxMin, roundTo, minSz)
			rmax1, rmax2 := idxSzRange(idxMax, roundTo, minSz)
			t.Fatalf("failed idx range to idx: "+
				"idx %d => [%d, %d]; idx %d => [%d, %d] "+
				"but for orig. idx %d "+
				"range [%d, %d], orig sz %d (roundTo %d, min %d)\n",
				idxMin, rmin1, rmin2,
				idxMax, rmax1, rmax2,
				idx, rmin, rmax, sz, roundTo, minSz)
		}
	}

}

func TestPoolOps(t *testing.T) {

	memLim := debug.SetMemoryLimit(-1)
	gcPcnt := debug.SetGCPercent(-1)
	debug.SetGCPercent(gcPcnt)
	t.Logf("system memory limit %d, GC Percent %d%%\n", memLim, gcPcnt)
	for n := 0; n < 1000; n++ {
		var bp Bpool

		minSz := int(rand.Int63n(128))
		maxSz := int(rand.Int63n(64*1024)) + 1024
		if maxSz <= minSz {
			maxSz = minSz + 1024
		}
		roundTo := int(rand.Int63n(32)) + 1

		if !bp.Init(minSz, maxSz, roundTo) {
			t.Fatalf("bpool init failed for %d, %d, %d\n",
				minSz, maxSz, roundTo)
		}
		t.Logf("%3d. using min size %d, max size %d, roundTo %d\n",
			n, minSz, maxSz, roundTo)

		allocs := make([][]byte, 1000)

		for i := 0; i < len(allocs); i++ {
			sz := int(rand.Int63n(2 * int64(maxSz)))
			// test no alloc
			b, ok := bp.Get(sz, false)
			if sz == 0 && (!ok || len(b) != 0) {
				t.Fatalf("bpool Get(%d, false) failed: [%d], %v\n",
					sz, len(b), ok)
			}
			if sz != 0 && (ok || len(b) != 0) {
				t.Fatalf("bpool Get(%d, false) failed: [%d], %v\n",
					sz, len(b), ok)
			}
			// test alloc
			b, ok = bp.Get(sz, true)
			if sz == 0 && (!ok || len(b) != 0) {
				t.Fatalf("bpool Get(%d, true) failed: [%d], %v\n",
					sz, len(b), ok)
			}
			if sz != 0 && (ok || len(b) != sz) {
				// should return a slice of len sz and false (not found
				// in the pool since we started with an empty one)
				t.Fatalf("bpool Get(%d, true) failed: [%d], %v"+
					" (min %d max %d roundTo %d)\n",
					sz, len(b), ok, minSz, maxSz, roundTo)
			}

			// test Put
			if sz != 0 && sz <= maxSz {
				runtime.SetFinalizer(&b[0], func(blk *byte) {
					t.Logf("finalizer for block %3d: %p size %d max %d"+
						" called\n",
						i, blk, sz, maxSz)
				})
			}
			// disable GC during the Put test
			// (we want to make sure that Get() returns something)
			gcPcnt = debug.SetGCPercent(-1)
			var gcStats1, gcStats2 debug.GCStats
			debug.ReadGCStats(&gcStats1)

			ok = bp.Put(b)
			if sz != 0 && sz <= maxSz && !ok {
				t.Fatalf("bpool Put([%d] len %d cap %d) failed:  %v"+
					" (min %d max %d roundTo %d)\n",
					sz, len(b), cap(b), ok, minSz, maxSz, roundTo)
			}
			if (sz > maxSz || sz == 0) && ok {
				t.Fatalf("bpool Put([%d] len %d cap %d) succeeded:  %v"+
					" (min %d max %d roundTo %d)\n",
					sz, len(b), cap(b), ok, minSz, maxSz, roundTo)
			}
			if sz != 0 && sz <= maxSz {
				// Get after Put
				b, ok = bp.Get(sz, false)
				debug.ReadGCStats(&gcStats2)
				if (gcStats2.LastGC != gcStats1.LastGC) ||
					(gcStats2.NumGC != gcStats1.NumGC) {
					t.Logf("GC run when gc disabled: "+
						" %d (%s) != %d (%s)\n",
						gcStats1.NumGC, gcStats1.LastGC,
						gcStats2.NumGC, gcStats2.LastGC)
				}
				if sz == 0 || !ok || len(b) != sz {
					// expected slice of len(sz) and true -- hit
					debug.ReadGCStats(&gcStats2)
					t.Fatalf("bpool Get(%d, false)  after Put failed:"+
						" [%d] cap %d, %v"+
						" (min %d max %d roundTo %d)"+
						" GC runs:  before put %d (%s)  after %d (%s)"+"\n",
						sz, len(b), cap(b), ok, minSz, maxSz, roundTo,
						gcStats1.NumGC, gcStats1.LastGC,
						gcStats2.NumGC, gcStats2.LastGC)
				}
				// save allocation
				allocs[i] = b
				runtime.SetFinalizer(&b[0], nil)
				// re-enable GC
				debug.SetGCPercent(gcPcnt)
			}
		} // end for ...len(allocs) -- alloc
		// Put/free all the in-use blocks
		for i := 0; i < len(allocs); i++ {
			if allocs[i] != nil {
				b := allocs[i]
				sz := len(b)
				ok := bp.Put(b)
				if sz != 0 && sz <= maxSz && !ok {
					t.Fatalf("bpool Put([%d] len %d cap %d) failed:  %v"+
						" (min %d max %d roundTo %d) for alloc %d\n",
						sz, len(b), cap(b), ok, minSz, maxSz, roundTo, i)
				}
				if (sz > maxSz || sz == 0) && ok {
					t.Fatalf("bpool Put([%d] len %d cap %d) succeeded:  %v"+
						" (min %d max %d roundTo %d) for alloc %d\n",
						sz, len(b), cap(b), ok, minSz, maxSz, roundTo, i)
				}
			}
		} // end for ..len(allocs) - free
		t.Logf("%3d. force GC (min size %d, max size %d, roundTo %d)\n",
			n, minSz, maxSz, roundTo)
		runtime.GC() // force GC run
	}
}
