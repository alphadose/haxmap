package haxmap

import (
	"encoding/binary"
	"math/bits"
	"reflect"
	"unsafe"

	"github.com/zeebo/xxh3"
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

	prime32_1 = 2654435761
	prime32_2 = 2246822519
	prime32_3 = 3266489917
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

var (
	// byte hasher, key size -> 1 byte
	byteHasher = func(key uint8) uintptr {
		return uintptr(_wx8(key))
	} // word hasher, key size -> 2 bytes
	wordHasher = func(key uint16) uintptr {
		return uintptr(_wx16(key))
	}

	// dword hasher, key size -> 4 bytes
	dwordHasher = func(key uint32) uintptr {
		return uintptr(_wx32(key))
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
		return uintptr((_wx64(key)))
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

	stringHasher = func(key string) uintptr {
		return uintptr(xxh3.HashString(key))
	}
)

func (m *Map[K, V]) setDefaultHasher() {
	// default hash functions
	switch reflect.TypeOf(*new(K)).Kind() {
	case reflect.String:
		m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&stringHasher))
		// use default xxHash algorithm for key of any size for golang string data type
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
	default:
		return

	}
}
