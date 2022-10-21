package benchmark

import (
	"sync/atomic"
	"testing"

	"github.com/alphadose/haxmap"
)

const (
	epochs  int64 = 1 << 12
	mapSize       = 256
)

func setupHaxMap() *haxmap.Map[int64, int64] {
	m := haxmap.New[int64, int64](mapSize)
	// m.SetHasher(phiMix2)
	for i := int64(0); i < epochs; i++ {
		m.Set(i, i)
	}
	return m
}

func setupIntIntMap() *Map {
	m := New(mapSize, 0.5)
	for i := int64(0); i < epochs; i++ {
		m.Put(i, i)
	}
	return m
}

func setupDefaultMap() map[int64]int64 {
	m := map[int64]int64{}
	for i := int64(0); i < epochs; i++ {
		m[i] = i
	}
	return m
}

func BenchmarkHaxMapReadsOnly(b *testing.B) {
	m := setupHaxMap()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := int64(0); i < epochs; i++ {
				j, _ := m.Get(i)
				if j != i {
					b.Fail()
				}
			}
		}
	})
}

func BenchmarkHaxMapReadsWithWrites(b *testing.B) {
	m := setupHaxMap()
	var writer uintptr
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// use 1 thread as writer
		if atomic.CompareAndSwapUintptr(&writer, 0, 1) {
			for pb.Next() {
				for i := int64(0); i < epochs; i++ {
					m.Set(i, i)
				}
			}
		} else {
			for pb.Next() {
				for i := int64(0); i < epochs; i++ {
					j, _ := m.Get(i)
					if j != i {
						b.Fail()
					}
				}
			}
		}
	})
}

func BenchmarkIntIntReadsOnly(b *testing.B) {
	m := setupIntIntMap()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := int64(0); i < epochs; i++ {
				j, _ := m.Get(i)
				if j != i {
					b.Fail()
				}
			}
		}
	})
}

func BenchmarkIntIntReadsWithWrites(b *testing.B) {
	m := setupIntIntMap()
	var writer uintptr
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// use 1 thread as writer
		if atomic.CompareAndSwapUintptr(&writer, 0, 1) {
			for pb.Next() {
				for i := int64(0); i < epochs; i++ {
					m.Put(i, i)
				}
			}
		} else {
			for pb.Next() {
				for i := int64(0); i < epochs; i++ {
					j, _ := m.Get(i)
					if j != i {
						b.Fail()
					}
				}
			}
		}
	})
}

func BenchmarkDefaultMapReadsOnly(b *testing.B) {
	m := setupDefaultMap()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := int64(0); i < epochs; i++ {
				j := m[i]
				if j != i {
					b.Fail()
				}
			}
		}
	})
}

// func BenchmarkDefaultMapReadsWithWrites(b *testing.B) {
// 	m := setupDefaultMap()
// 	var writer uintptr
// 	b.ResetTimer()
// 	b.RunParallel(func(pb *testing.PB) {
// 		// use 1 thread as writer
// 		if atomic.CompareAndSwapUintptr(&writer, 0, 1) {
// 			for pb.Next() {
// 				for i := int64(0); i < epochs; i++ {
// 					m[i] = i
// 				}
// 			}
// 		} else {
// 			for pb.Next() {
// 				for i := int64(0); i < epochs; i++ {
// 					j := m[i]
// 					if j != i {
// 						b.Fail()
// 					}
// 				}
// 			}
// 		}
// 	})
// }
