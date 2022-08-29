package haxmap

import (
	"sync/atomic"
	"testing"
)

const (
	epochs  uintptr = 1 << 12
	mapSize         = 256
)

func setupHaxMap() *HashMap[uintptr, uintptr] {
	m := New[uintptr, uintptr](mapSize)
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
