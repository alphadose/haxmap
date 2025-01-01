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

	key64_000 uint64 = 0xbe4ba423396cfeb8
	key64_008 uint64 = 0x1cad21f72c81017c
	key64_016 uint64 = 0xdb979083e96dd4de
	key64_024 uint64 = 0x1f67b3b7a4a44072
	key64_032 uint64 = 0x78e5c0cc4ee679cb
	key64_040 uint64 = 0x2172ffcc7dd05a82
	key64_048 uint64 = 0x8e2443f7744608b8
	key64_056 uint64 = 0x4c263a81e69035e0
	key64_064 uint64 = 0xcb00c391bb52283c
	key64_072 uint64 = 0xa32e531b8b65d088
	key64_080 uint64 = 0x4ef90da297486471
	key64_088 uint64 = 0xd8acdea946ef1938
	key64_096 uint64 = 0x3f349ce33f76faa8
	key64_104 uint64 = 0x1d4f0bc7c7bbdcf9
	key64_112 uint64 = 0x3159b4cd4be0518a
	key64_120 uint64 = 0x647378d9c97e9fc8
	key64_128 uint64 = 0xc3ebd33483acc5ea
	key64_136 uint64 = 0xeb6313faffa081c5
	key64_144 uint64 = 0x49daf0b751dd0d17
	key64_152 uint64 = 0x9e68d429265516d3
	key64_160 uint64 = 0xfca1477d58be162b
	key64_168 uint64 = 0xce31d07ad1b8f88f
	key64_176 uint64 = 0x280416958f3acb45
	key64_184 uint64 = 0x7e404bbbcafbd7af

	key64_103 uint64 = 0x4f0bc7c7bbdcf93f
	key64_111 uint64 = 0x59b4cd4be0518a1d
	key64_119 uint64 = 0x7378d9c97e9fc831
	key64_127 uint64 = 0xebd33483acc5ea64

	key64_121 uint64 = 0xea647378d9c97e9f
	key64_129 uint64 = 0xc5c3ebd33483acc5
	key64_137 uint64 = 0x17eb6313faffa081
	key64_145 uint64 = 0xd349daf0b751dd0d
	key64_153 uint64 = 0x2b9e68d429265516
	key64_161 uint64 = 0x8ffca1477d58be16
	key64_169 uint64 = 0x45ce31d07ad1b8f8
	key64_177 uint64 = 0xaf280416958f3acb

	key64_011 = 0x6dd4de1cad21f72c
	key64_019 = 0xa44072db979083e9
	key64_027 = 0xe679cb1f67b3b7a4
	key64_035 = 0xd05a8278e5c0cc4e
	key64_043 = 0x4608b82172ffcc7d
	key64_051 = 0x9035e08e2443f774
	key64_059 = 0x52283c4c263a81e6
	key64_067 = 0x65d088cb00c391bb

	key64_117 = 0xd9c97e9fc83159b4
	key64_125 = 0x3483acc5ea647378
	key64_133 = 0xfaffa081c5c3ebd3
	key64_141 = 0xb751dd0d17eb6313
	key64_149 = 0x29265516d349daf0
	key64_157 = 0x7d58be162b9e68d4
	key64_165 = 0x7ad1b8f88ffca147
	key64_173 = 0x958f3acb45ce31d0

	key32_000 uint32 = 0xbe4ba423
	key32_004 uint32 = 0x396cfeb8
	key32_008 uint32 = 0x1cad21f7
	key32_012 uint32 = 0x2c81017c
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
	byteHasher = func(key uint8) (acc uint64) {
		acc = uint64(key)
		acc = acc*(1<<24+1<<16+1) + 1<<8
		acc ^= uint64(key32_000 ^ key32_004)

		return xxhAvalancheSmall(acc)
	}

	// word hasher, key size -> 2 bytes
	wordHasher = func(key uint16) (acc uint64) {
		key = readU16(ptr(&key), 0)
		acc = uint64(key)*(1<<24+1)>>8 + 2<<8

		acc ^= uint64(key32_000 ^ key32_004)

		return xxhAvalancheSmall(acc)
	}

	// dword hasher, key size -> 4 bytes
	dwordHasher = func(key uint32) (acc uint64) {
		key = readU32(ptr(&key), 0)
		input2 := readU32(ptr(&key), uintptr(key)-4)
		acc = uint64(input2) + uint64(key)<<32
		acc = acc ^ (key64_008 ^ key64_016)
		return rrmxmx(acc, uint64(key))
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
	qwordHasher = func(key uint64) (acc uint64) {
		inputlo := readU64(ptr(&key), 0) ^ (key64_024 ^ key64_032)
		inputhi := bits.ReverseBytes64(key) ^ key64_040
		folded := mulFold64(inputlo, inputhi)

		acc = xxh3Avalanche(inputlo + inputhi + folded)
		return acc
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

	stringHasher = func(key string) uint64 {
		return xxh3.HashString(key)
	}
)

func (m *Map[K, V]) setDefaultHasher() {
	// default hash functions
	switch reflect.TypeOf(*new(K)).Kind() {
	case reflect.String:
		// use  xxHash3 algorithm for key of any size for golang string data type
		m.hasher = *(*func(K) uintptr)(unsafe.Pointer(&stringHasher))

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
		panic("unsupported key type")

	}
}
