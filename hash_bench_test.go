package haxmap

import (
	"math/bits"
	"reflect"
	"strconv"
	"testing"
	"unsafe"
)

const numbeF = 10000

var uint8HasherDefault = func(key uint8) uintptr {
	h := prime5 + 1
	h ^= uint64(key) * prime5
	h = bits.RotateLeft64(h, 11) * prime1

	h ^= h >> 33
	h *= prime2
	h ^= h >> 29
	h *= prime3
	h ^= h >> 32
	return uintptr(h)
}

var uint64HasherDefault = func(key uint64) uintptr {
	h := prime5 + 8
	h ^= key * prime5
	h = bits.RotateLeft64(h, 27)*prime1 + prime4
	h ^= h >> 33
	h *= prime2
	h ^= h >> 29
	h *= prime3
	h ^= h >> 32
	return uintptr(h)
}

var stringAnotherHash = func(key string) uintptr {
	strHeader := (*reflect.StringHeader)(unsafe.Pointer(&key))
	data := unsafe.Pointer(strHeader.Data)
	length := strHeader.Len
	var h uint64 = prime5 + uint64(length)

	for length >= 8 {
		h ^= u64(*(*[]byte)(data)) * prime2
		h = bits.RotateLeft64(h, 31) * prime1
		length -= 8
		data = unsafe.Add(data, 8)
	}

	for i := 0; i < length; i++ {
		h ^= uint64(*(*byte)(unsafe.Add(data, i))) * prime5
		h = bits.RotateLeft64(h, 11) * prime1
	}

	h ^= h >> 33
	h *= prime2
	h ^= h >> 29
	h *= prime3
	h ^= h >> 32

	return uintptr(h)
}

var stringDefaultXXHASH = func(key string) uintptr {

	sh := (*reflect.StringHeader)(unsafe.Pointer(&key))
	b := unsafe.Slice((*byte)(unsafe.Pointer(sh.Data)), sh.Len)
	n := sh.Len
	var h uint64

	if n >= 32 {
		v1 := prime1v + prime2
		v2 := prime2
		v3 := uint64(0)
		v4 := -prime1v
		for len(b) >= 32 {
			v1 = round(v1, u64(b[0:8:len(b)]))
			v2 = round(v2, u64(b[8:16:len(b)]))
			v3 = round(v3, u64(b[16:24:len(b)]))
			v4 = round(v4, u64(b[24:32:len(b)]))
			b = b[32:len(b):len(b)]
		}
		h = rol1(v1) + rol7(v2) + rol12(v3) + rol18(v4)
		h = mergeRound(h, v1)
		h = mergeRound(h, v2)
		h = mergeRound(h, v3)
		h = mergeRound(h, v4)
	} else {
		h = prime5
	}

	h += uint64(n)

	i, end := 0, len(b)
	for ; i+8 <= end; i += 8 {
		k1 := round(0, u64(b[i:i+8:len(b)]))
		h ^= k1
		h = rol27(h)*prime1 + prime4
	}
	if i+4 <= end {
		h ^= uint64(u32(b[i:i+4:len(b)])) * prime1
		h = rol23(h)*prime2 + prime3
		i += 4
	}
	for ; i < end; i++ {
		h ^= uint64(b[i]) * prime5
		h = rol11(h) * prime1
	}

	h ^= h >> 33
	h *= prime2
	h ^= h >> 29
	h *= prime3
	h ^= h >> 32

	return uintptr(h)
}

func BenchmarkTestStringHash(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {

		m := New[string, string](0)
		for pb.Next() {

			for i := 0; i < numbeF; i++ {

				m.Set(strconv.Itoa(i), strconv.Itoa(i))

			}

			for i := 0; i < numbeF; i++ {

				m.Get(strconv.Itoa(i))

			}

		}
	})

}

func BenchmarkTestStringHash2(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {

		m := New[string, string](0)
		m.SetHasher(stringDefaultXXHASH)

		for pb.Next() {
			for i := 0; i < numbeF; i++ {

				m.Set(strconv.Itoa(i), strconv.Itoa(i))

			}

			for i := 0; i < numbeF; i++ {
				m.Del(strconv.Itoa(i))
			}
		}

	})
}

func BenchmarkTestStringHash3(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {

		m := New[string, string](0)
		m.SetHasher(stringAnotherHash)

		for pb.Next() {
			for i := 0; i < numbeF; i++ {

				m.Set(strconv.Itoa(i), strconv.Itoa(i))

			}

			for i := 0; i < numbeF; i++ {
				m.Del(strconv.Itoa(i))
			}
		}

	})
}

func BenchmarkTestUnt8Hash(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {

		m := New[uint8, uint8](0)
		for pb.Next() {

			for i := 0; i < numbeF; i++ {

				m.Set(uint8(i), uint8(i))

			}

			for i := 0; i < numbeF; i++ {
				m.Get(uint8(i))
			}

		}
	})
}

func BenchmarkTestUint8HashDefault(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {

		m := New[uint8, uint8](0)
		m.SetHasher(uint8HasherDefault)

		for pb.Next() {
			for i := 0; i < numbeF; i++ {

				m.Set(uint8(i), uint8(i))

			}

			for i := 0; i < numbeF; i++ {
				m.Del(uint8(i))
			}
		}

	})
}

func BenchmarkTestUint64Hash(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		m := New[uint64, uint64](0)

		for pb.Next() {
			for i := 0; i < numbeF; i++ {

				m.Set(uint64(i), uint64(i))

			}

			for i := 0; i < numbeF; i++ {
				m.Del(uint64(i))
			}
		}

	})
}

func BenchmarkTestUint64HashDefault(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		m := New[uint64, uint64](0)
		m.SetHasher(uint64HasherDefault)

		for pb.Next() {
			for i := 0; i < numbeF; i++ {

				m.Set(uint64(i), uint64(i))

			}

			for i := 0; i < numbeF; i++ {
				m.Del(uint64(i))
			}
		}

	})
}
