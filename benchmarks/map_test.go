package benchmark

import (
	"hash/maphash"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/alphadose/haxmap"
)

const (
	epochs  int = 1 << 12
	mapSize     = 256
)

func setupXXHash() *haxmap.Map[int, int] {
	m := haxmap.New[int, int](mapSize)
	for i := int(0); i < epochs; i++ {
		m.Set(i, i)
	}
	return m
}

var seed = maphash.MakeSeed()

func maphashInt(i int) uintptr {
	return uintptr(maphash.Bytes(seed, (*(*[8]byte)(unsafe.Pointer(&i)))[:]))
}

// https://github.com/golang/go/blob/master/src/hash/maphash/maphash.go
func setupMapHash() *haxmap.Map[int, int] {
	m := haxmap.New[int, int](mapSize)
	m.SetHasher(maphashInt)
	for i := int(0); i < epochs; i++ {
		m.Set(i, i)
	}
	return m
}

func BenchmarkXXHashReadsOnly(b *testing.B) {
	m := setupXXHash()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := int(0); i < epochs; i++ {
				j, _ := m.Get(i)
				if j != i {
					b.Fail()
				}
			}
		}
	})
}

func BenchmarkXXHashReadsWithWrites(b *testing.B) {
	m := setupXXHash()
	var writer uintptr
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// use 1 thread as writer
		if atomic.CompareAndSwapUintptr(&writer, 0, 1) {
			for pb.Next() {
				for i := int(0); i < epochs; i++ {
					m.Set(i, i)
				}
			}
		} else {
			for pb.Next() {
				for i := int(0); i < epochs; i++ {
					j, _ := m.Get(i)
					if j != i {
						b.Fail()
					}
				}
			}
		}
	})
}

func BenchmarkMapHashReadsOnly(b *testing.B) {
	m := setupMapHash()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := int(0); i < epochs; i++ {
				j, _ := m.Get(i)
				if j != i {
					b.Fail()
				}
			}
		}
	})
}

func BenchmarkMapHashReadsWithWrites(b *testing.B) {
	m := setupMapHash()
	var writer uintptr
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// use 1 thread as writer
		if atomic.CompareAndSwapUintptr(&writer, 0, 1) {
			for pb.Next() {
				for i := int(0); i < epochs; i++ {
					m.Set(i, i)
				}
			}
		} else {
			for pb.Next() {
				for i := int(0); i < epochs; i++ {
					j, _ := m.Get(i)
					if j != i {
						b.Fail()
					}
				}
			}
		}
	})
}
