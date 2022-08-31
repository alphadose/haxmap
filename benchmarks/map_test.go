package benchmark

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alphadose/haxmap"
	"github.com/cornelk/hashmap"
)

const (
	epochs  uintptr = 1 << 12
	mapSize         = 256
)

func setupHaxMap() *haxmap.HashMap[uintptr, uintptr] {
	m := haxmap.New[uintptr, uintptr](mapSize)
	for i := uintptr(0); i < epochs; i++ {
		m.Set(i, i)
	}
	return m
}

func setupGoSyncMap() *sync.Map {
	m := &sync.Map{}
	for i := uintptr(0); i < epochs; i++ {
		m.Store(i, i)
	}
	return m
}

func setupCornelkMap(b *testing.B) *hashmap.Map[uintptr, uintptr] {
	m := hashmap.NewSized[uintptr, uintptr](mapSize)
	for i := uintptr(0); i < epochs; i++ {
		m.Set(i, i)
	}
	return m
}

func BenchmarkHaxMapReadsOnly(b *testing.B) {
	m := setupHaxMap()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := uintptr(0); i < epochs; i++ {
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
				for i := uintptr(0); i < epochs; i++ {
					m.Set(i, i)
				}
			}
		} else {
			for pb.Next() {
				for i := uintptr(0); i < epochs; i++ {
					j, _ := m.Get(i)
					if j != i {
						b.Fail()
					}
				}
			}
		}
	})
}

func BenchmarkGoSyncMapReadsOnly(b *testing.B) {
	m := setupGoSyncMap()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := uintptr(0); i < epochs; i++ {
				j, _ := m.Load(i)
				if j != i {
					b.Fail()
				}
			}
		}
	})
}

func BenchmarkGoSyncMapReadsWithWrites(b *testing.B) {
	m := setupGoSyncMap()
	var writer uintptr
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// use 1 thread as writer
		if atomic.CompareAndSwapUintptr(&writer, 0, 1) {
			for pb.Next() {
				for i := uintptr(0); i < epochs; i++ {
					m.Store(i, i)
				}
			}
		} else {
			for pb.Next() {
				for i := uintptr(0); i < epochs; i++ {
					j, _ := m.Load(i)
					if j != i {
						b.Fail()
					}
				}
			}
		}
	})
}

func BenchmarkCornelkMapReadsOnly(b *testing.B) {
	m := setupCornelkMap(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := uintptr(0); i < epochs; i++ {
				j, _ := m.Get(i)
				if j != i {
					b.Fail()
				}
			}
		}
	})
}

func BenchmarkCornelkMapReadsWithWrites(b *testing.B) {
	m := setupCornelkMap(b)
	var writer uintptr
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// use 1 thread as writer
		if atomic.CompareAndSwapUintptr(&writer, 0, 1) {
			for pb.Next() {
				for i := uintptr(0); i < epochs; i++ {
					m.Set(i, i)
				}
			}
		} else {
			for pb.Next() {
				for i := uintptr(0); i < epochs; i++ {
					j, _ := m.Get(i)
					if j != i {
						b.Fail()
					}
				}
			}
		}
	})
}
