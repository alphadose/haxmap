//go:build amd64 || arm64 || mips64 || mips64le || ppc64 || ppc64le || riscv64 || s390x || wasm

/*
From https://github.com/cespare/xxhash

Copyright (c) 2016 Caleb Spare

MIT License

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:
The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package haxmap

import (
	"encoding/binary"
	"math/bits"
	"reflect"
	"unsafe"
)

const (
	prime1 uint64 = 11400714785074694791
	prime2 uint64 = 14029467366897019727
	prime3 uint64 = 1609587929392839161
	prime4 uint64 = 9650029242287828579
	prime5 uint64 = 2870177450012600261
)

var prime1v = prime1

// defaultSum implements xxHash for 64 bit systems
func defaultSum(b []byte) uintptr {
	n := len(b)
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

func u64(b []byte) uint64 { return binary.LittleEndian.Uint64(b) }
func u32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

func round(acc, input uint64) uint64 {
	acc += input * prime2
	acc = rol31(acc)
	acc *= prime1
	return acc
}

func mergeRound(acc, val uint64) uint64 {
	val = round(0, val)
	acc ^= val
	acc = acc*prime1 + prime4
	return acc
}

func rol1(x uint64) uint64  { return bits.RotateLeft64(x, 1) }
func rol7(x uint64) uint64  { return bits.RotateLeft64(x, 7) }
func rol11(x uint64) uint64 { return bits.RotateLeft64(x, 11) }
func rol12(x uint64) uint64 { return bits.RotateLeft64(x, 12) }
func rol18(x uint64) uint64 { return bits.RotateLeft64(x, 18) }
func rol23(x uint64) uint64 { return bits.RotateLeft64(x, 23) }
func rol27(x uint64) uint64 { return bits.RotateLeft64(x, 27) }
func rol31(x uint64) uint64 { return bits.RotateLeft64(x, 31) }

func (m *HashMap[K, V]) setDefaultHasher() {
	// default hash functions
	// xxHash implementation for known key type sizes
	// minimal hash functions with no branching
	// inline hash function assignment for better performance

	switch any(*new(K)).(type) {
	case string:
		m.hasher = func(key K) uintptr {
			sh := (*reflect.StringHeader)(unsafe.Pointer(&key))
			return uintptr(defaultSum(unsafe.Slice((*byte)(unsafe.Pointer(&key)), sh.Len)))
		}
	case int, uint, uintptr:
		switch intSizeBytes {
		case 2:
			// word hasher
			m.hasher = func(key K) uintptr {
				b := unsafe.Slice((*byte)(unsafe.Pointer(&key)), wordSize)

				var h = prime5 + 2

				h ^= uint64(b[0]) * prime5
				h = bits.RotateLeft64(h, 11) * prime1
				h ^= uint64(b[1]) * prime5
				h = bits.RotateLeft64(h, 11) * prime1

				h ^= h >> 33
				h *= prime2
				h ^= h >> 29
				h *= prime3
				h ^= h >> 32

				return uintptr(h)
			}
		case 4:
			// Dword hasher
			m.hasher = func(key K) uintptr {
				b := unsafe.Slice((*byte)(unsafe.Pointer(&key)), dwordSize)

				var h = prime5 + 4
				h ^= (uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24) * prime1
				h = bits.RotateLeft64(h, 23)*prime2 + prime3

				h ^= h >> 33
				h *= prime2
				h ^= h >> 29
				h *= prime3
				h ^= h >> 32

				return uintptr(h)
			}
		case 8:
			// Qword Hash
			m.hasher = func(key K) uintptr {
				b := unsafe.Slice((*byte)(unsafe.Pointer(&key)), qwordSize)

				var h = prime5 + 8

				val := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
					uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56

				k1 := val * prime2
				k1 = bits.RotateLeft64(k1, 31)
				k1 *= prime1

				h ^= k1
				h = bits.RotateLeft64(h, 27)*prime1 + prime4

				h ^= h >> 33
				h *= prime2
				h ^= h >> 29
				h *= prime3
				h ^= h >> 32

				return uintptr(h)
			}
		}
	case int8, uint8:
		// byte word hasher
		m.hasher = func(key K) uintptr {
			b := unsafe.Slice((*byte)(unsafe.Pointer(&key)), byteSize)

			var h = prime5 + 1
			h ^= uint64(b[0]) * prime5
			h = bits.RotateLeft64(h, 11) * prime1

			h ^= h >> 33
			h *= prime2
			h ^= h >> 29
			h *= prime3
			h ^= h >> 32

			return uintptr(h)
		}

	case int16, uint16:
		// word hasher
		m.hasher = func(key K) uintptr {
			b := unsafe.Slice((*byte)(unsafe.Pointer(&key)), wordSize)

			var h = prime5 + 2

			h ^= uint64(b[0]) * prime5
			h = bits.RotateLeft64(h, 11) * prime1
			h ^= uint64(b[1]) * prime5
			h = bits.RotateLeft64(h, 11) * prime1

			h ^= h >> 33
			h *= prime2
			h ^= h >> 29
			h *= prime3
			h ^= h >> 32

			return uintptr(h)
		}
	case int32, uint32, float32:
		// Dword hasher
		m.hasher = func(key K) uintptr {
			b := unsafe.Slice((*byte)(unsafe.Pointer(&key)), dwordSize)

			var h = prime5 + 4
			h ^= (uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24) * prime1
			h = bits.RotateLeft64(h, 23)*prime2 + prime3

			h ^= h >> 33
			h *= prime2
			h ^= h >> 29
			h *= prime3
			h ^= h >> 32

			return uintptr(h)
		}
	case int64, uint64, float64, complex64:
		// Qword hasher
		m.hasher = func(key K) uintptr {
			b := unsafe.Slice((*byte)(unsafe.Pointer(&key)), qwordSize)

			var h = prime5 + 8

			val := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
				uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56

			k1 := val * prime2
			k1 = bits.RotateLeft64(k1, 31)
			k1 *= prime1

			h ^= k1
			h = bits.RotateLeft64(h, 27)*prime1 + prime4

			h ^= h >> 33
			h *= prime2
			h ^= h >> 29
			h *= prime3
			h ^= h >> 32

			return uintptr(h)
		}
	case complex128:
		// Oword hasher
		m.hasher = func(key K) uintptr {
			b := unsafe.Slice((*byte)(unsafe.Pointer(&key)), owordSize)

			var h = prime5 + 16

			val := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
				uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56

			k1 := val * prime2
			k1 = bits.RotateLeft64(k1, 31)
			k1 *= prime1

			h ^= k1
			h = bits.RotateLeft64(h, 27)*prime1 + prime4

			val = uint64(b[8]) | uint64(b[9])<<8 | uint64(b[10])<<16 | uint64(b[11])<<24 |
				uint64(b[12])<<32 | uint64(b[13])<<40 | uint64(b[14])<<48 | uint64(b[15])<<56

			k1 = val * prime2
			k1 = bits.RotateLeft64(k1, 31)
			k1 *= prime1

			h ^= k1
			h = bits.RotateLeft64(h, 27)*prime1 + prime4

			h ^= h >> 33
			h *= prime2
			h ^= h >> 29
			h *= prime3
			h ^= h >> 32

			return uintptr(h)
		}
	}
}
