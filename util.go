package haxmap

import (
	"math/bits"
	"unsafe"
)

var key = ptr(&[...]uint8{
	0xb8, 0xfe, 0x6c, 0x39, 0x23, 0xa4, 0x4b, 0xbe /* 8   */, 0x7c, 0x01, 0x81, 0x2c, 0xf7, 0x21, 0xad, 0x1c, /* 16  */
	0xde, 0xd4, 0x6d, 0xe9, 0x83, 0x90, 0x97, 0xdb /* 24  */, 0x72, 0x40, 0xa4, 0xa4, 0xb7, 0xb3, 0x67, 0x1f, /* 32  */
	0xcb, 0x79, 0xe6, 0x4e, 0xcc, 0xc0, 0xe5, 0x78 /* 40  */, 0x82, 0x5a, 0xd0, 0x7d, 0xcc, 0xff, 0x72, 0x21, /* 48  */
	0xb8, 0x08, 0x46, 0x74, 0xf7, 0x43, 0x24, 0x8e /* 56  */, 0xe0, 0x35, 0x90, 0xe6, 0x81, 0x3a, 0x26, 0x4c, /* 64  */
	0x3c, 0x28, 0x52, 0xbb, 0x91, 0xc3, 0x00, 0xcb /* 72  */, 0x88, 0xd0, 0x65, 0x8b, 0x1b, 0x53, 0x2e, 0xa3, /* 80  */
	0x71, 0x64, 0x48, 0x97, 0xa2, 0x0d, 0xf9, 0x4e /* 88  */, 0x38, 0x19, 0xef, 0x46, 0xa9, 0xde, 0xac, 0xd8, /* 96  */
	0xa8, 0xfa, 0x76, 0x3f, 0xe3, 0x9c, 0x34, 0x3f /* 104 */, 0xf9, 0xdc, 0xbb, 0xc7, 0xc7, 0x0b, 0x4f, 0x1d, /* 112 */
	0x8a, 0x51, 0xe0, 0x4b, 0xcd, 0xb4, 0x59, 0x31 /* 120 */, 0xc8, 0x9f, 0x7e, 0xc9, 0xd9, 0x78, 0x73, 0x64, /* 128 */
	0xea, 0xc5, 0xac, 0x83, 0x34, 0xd3, 0xeb, 0xc3 /* 136 */, 0xc5, 0x81, 0xa0, 0xff, 0xfa, 0x13, 0x63, 0xeb, /* 144 */
	0x17, 0x0d, 0xdd, 0x51, 0xb7, 0xf0, 0xda, 0x49 /* 152 */, 0xd3, 0x16, 0x55, 0x26, 0x29, 0xd4, 0x68, 0x9e, /* 160 */
	0x2b, 0x16, 0xbe, 0x58, 0x7d, 0x47, 0xa1, 0xfc /* 168 */, 0x8f, 0xf8, 0xb8, 0xd1, 0x7a, 0xd0, 0x31, 0xce, /* 176 */
	0x45, 0xcb, 0x3a, 0x8f, 0x95, 0x16, 0x04, 0x28 /* 184 */, 0xaf, 0xd7, 0xfb, 0xca, 0xbb, 0x4b, 0x40, 0x7e, /* 192 */
})

type Uint128 struct {
	Hi, Lo uint64
}

// Bytes returns the uint128 as an array of bytes in canonical form (big-endian encoded).
func (u Uint128) Bytes() [16]byte {
	return [16]byte{
		byte(u.Hi >> 0x38), byte(u.Hi >> 0x30), byte(u.Hi >> 0x28), byte(u.Hi >> 0x20),
		byte(u.Hi >> 0x18), byte(u.Hi >> 0x10), byte(u.Hi >> 0x08), byte(u.Hi),
		byte(u.Lo >> 0x38), byte(u.Lo >> 0x30), byte(u.Lo >> 0x28), byte(u.Lo >> 0x20),
		byte(u.Lo >> 0x18), byte(u.Lo >> 0x10), byte(u.Lo >> 0x08), byte(u.Lo),
	}
}

type (
	ptr = unsafe.Pointer
)

type str struct {
	p ptr
	l uint
}

func readU8(p ptr, o uintptr) uint8 {
	return *(*uint8)(ptr(uintptr(p) + o))
}

func readU16(p ptr, o uintptr) uint16 {
	b := (*[2]byte)(ptr(uintptr(p) + o))
	return Uint16(b)
}

func readU32(p ptr, o uintptr) uint32 {
	b := (*[4]byte)(ptr(uintptr(p) + o))
	return Uint32(b)
}

func readU64(p ptr, o uintptr) uint64 {
	b := (*[8]byte)(ptr(uintptr(p) + o))
	return Uint64(b)
}

func Uint16(b *[2]byte) uint16 {
	return uint16(b[0]) | uint16(b[1])<<8
}

func Uint32(b *[4]byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func Uint64(b *[8]byte) uint64 {
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<5
}

func writeU64(p ptr, o uintptr, v uint64) {
	b := (*[8]byte)(ptr(uintptr(p) + o))
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}

const secretSize = 192

func initSecret(secret unsafe.Pointer, seed uint64) {
	for i := uintptr(0); i < secretSize/16; i++ {
		lo := readU64(key, 16*i) + seed
		hi := readU64(key, 16*i+8) - seed
		writeU64(secret, 16*i, lo)
		writeU64(secret, 16*i+8, hi)
	}
}

func xxh64AvalancheSmall(x uint64) uint64 {
	// x ^= x >> 33                    // x must be < 32 bits
	// x ^= u64(key32_000 ^ key32_004) // caller must do this
	x *= prime2
	x ^= x >> 29
	x *= prime3
	x ^= x >> 32
	return x
}

func xxhAvalancheSmall(x uint64) uint64 {
	x ^= x >> 33
	x *= prime2
	x ^= x >> 29
	x *= prime3
	x ^= x >> 32
	return x
}

func xxh64AvalancheFull(x uint64) uint64 {
	x ^= x >> 33
	x *= prime2
	x ^= x >> 29
	x *= prime3
	x ^= x >> 32
	return x
}

func xxh3Avalanche(x uint64) uint64 {
	x ^= x >> 37
	x *= 0x165667919e3779f9
	x ^= x >> 32
	return x
}

func rrmxmx(h64 uint64, len uint64) uint64 {
	h64 ^= bits.RotateLeft64(h64, 49) ^ bits.RotateLeft64(h64, 24)
	h64 *= 0x9fb21c651e98df25
	h64 ^= (h64 >> 35) + len
	h64 *= 0x9fb21c651e98df25
	h64 ^= (h64 >> 28)
	return h64
}

func mulFold64(x, y uint64) uint64 {
	hi, lo := bits.Mul64(x, y)
	return hi ^ lo
}
