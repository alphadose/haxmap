package haxmap

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

import (
	"encoding/binary"
	"math/bits"
	"reflect"
	"unsafe"
)

const (
	// hash input allowed sizes
	byteSize = 1 << iota
	wordSize
	dwordSize
	qwordSize
	owordSize
)

const (
	prime1 uint64 = 11400714785074694791
	prime2 uint64 = 14029467366897019727
	prime3 uint64 = 1609587929392839161
	prime4 uint64 = 9650029242287828579
	prime5 uint64 = 2870177450012600261
)

var prime1v = prime1

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

// xxHash implementation for known key type sizes, minimal with no branching
var (
	// byte hasher, key size -> 1 byte
	byteHasher = func(key uint8) uintptr {
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

	// word hasher, key size -> 2 bytes
	wordHasher = func(key uint16) uintptr {
		h := prime5 + 2
		h ^= (uint64(key) & 0xff) * prime5
		h = bits.RotateLeft64(h, 11) * prime1
		h ^= ((uint64(key) >> 8) & 0xff) * prime5
		h = bits.RotateLeft64(h, 11) * prime1
		h ^= h >> 33
		h *= prime2
		h ^= h >> 29
		h *= prime3
		h ^= h >> 32
		return uintptr(h)
	}

	// dword hasher, key size -> 4 bytes
	dwordHasher = func(key uint32) uintptr {
		h := prime5 + 4
		h ^= uint64(key) * prime1
		h = bits.RotateLeft64(h, 23)*prime2 + prime3
		h ^= h >> 33
		h *= prime2
		h ^= h >> 29
		h *= prime3
		h ^= h >> 32
		return uintptr(h)
	}

	// separate dword hasher for float32 type
	// required for casting float32 to unsigned integer type without any loss of bits
	// Example :- casting uint32(1.3) will drop off the 0.3 decimal part but using *(*uint32)(unsafe.Pointer(&key)) will retain all bits (both the integer as well as the decimal part)
	// this will ensure correctness of the hash
	float32Hasher = func(key float32) uintptr {
		h := prime5 + 4
		h ^= uint64(*(*uint32)(unsafe.Pointer(&key))) * prime1
		h = bits.RotateLeft64(h, 23)*prime2 + prime3
		h ^= h >> 33
		h *= prime2
		h ^= h >> 29
		h *= prime3
		h ^= h >> 32
		return uintptr(h)
	}

	// qword hasher, key size -> 8 bytes
	qwordHasher = func(key uint64) uintptr {
		k1 := key * prime2
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= prime1
		h := (prime5 + 8) ^ k1
		h = bits.RotateLeft64(h, 27)*prime1 + prime4
		h ^= h >> 33
		h *= prime2
		h ^= h >> 29
		h *= prime3
		h ^= h >> 32
		return uintptr(h)
	}

	// separate qword hasher for float64 type
	// for reason see definition of float32Hasher on line 127
	float64Hasher = func(key float64) uintptr {
		k1 := *(*uint64)(unsafe.Pointer(&key)) * prime2
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= prime1
		h := (prime5 + 8) ^ k1
		h = bits.RotateLeft64(h, 27)*prime1 + prime4
		h ^= h >> 33
		h *= prime2
		h ^= h >> 29
		h *= prime3
		h ^= h >> 32
		return uintptr(h)
	}

	// separate qword hasher for complex64 type
	complex64Hasher = func(key complex64) uintptr {
		k1 := *(*uint64)(unsafe.Pointer(&key)) * prime2
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= prime1
		h := (prime5 + 8) ^ k1
		h = bits.RotateLeft64(h, 27)*prime1 + prime4
		h ^= h >> 33
		h *= prime2
		h ^= h >> 29
		h *= prime3
		h ^= h >> 32
		return uintptr(h)
	}
)

func (m *Map[K, V]) setDefaultHasher() {
	// default hash functions
	switch reflect.TypeOf(*new(K)).Kind() {
	case reflect.String:
		// use default xxHash algorithm for key of any size for golang string data type
		m.hasher = func(key K) uintptr {
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
	case reflect.Int, reflect.Uint, reflect.Uintptr, reflect.UnsafePointer:
		switch intSizeBytes {
		case 2:
			// word hasher
			m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&wordHasher))
		case 4:
			// dword hasher
			m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&dwordHasher))
		case 8:
			// qword hasher
			m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&qwordHasher))
		}
	case reflect.Int8, reflect.Uint8:
		// byte hasher
		m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&byteHasher))
	case reflect.Int16, reflect.Uint16:
		// word hasher
		m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&wordHasher))
	case reflect.Int32, reflect.Uint32:
		// dword hasher
		m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&dwordHasher))
	case reflect.Float32:
		// custom float32 dword hasher
		m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&float32Hasher))
	case reflect.Int64, reflect.Uint64:
		// qword hasher
		m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&qwordHasher))
	case reflect.Float64:
		// custom float64 qword hasher
		m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&float64Hasher))
	case reflect.Complex64:
		// custom complex64 qword hasher
		m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&complex64Hasher))
	case reflect.Complex128:
		// oword hasher, key size -> 16 bytes
		m.hasher = func(key K) uintptr {
			b := *(*[owordSize]byte)(unsafe.Pointer(&key))
			h := prime5 + 16

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
