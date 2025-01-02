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

const (
	_wyp0 = 0xa0761d6478bd642f
	_wyp1 = 0xe7037ed1a0b428db
	_wyp2 = 0x8ebc6af09c88c6e3
	_wyp3 = 0x589965cc75374cc3
	_wyp4 = 0x1d8e4e27c47d124f
)

var _wyp_a = [4]uint64{
	0x2d358dccaa6c78a5,
	0x8bb84b93962eacc9,
	0x4b33a62ed433d4a3,
	0x4d5a2da51de1aa47,
}

type (
	ptr = unsafe.Pointer
)

type str struct {
	p ptr
	l uint
}

//
// //go:nosplit
// //go:nocheckptr
// func noescape(up ptr) ptr {
// 	x := uintptr(up)
// 	return ptr(x ^ 0)
// }

//go:nosplit
//go:nocheckptr
func off(p ptr, n uintptr) ptr { return ptr(uintptr(p) + n) }

func _wymix(a, key uint64) uint64 {
	return _wmum(a^key^_wyp0, key^_wyp1)
}

func _wx10(key uint64) uint64 {
	key += _wyp0
	return _wmum(uint64(key)^_wyp1, uint64(key))

}
func _wx64(key uint64) uint64 { // 8 byte
	p := ptr(&key)

	return _wmum(_wmum(_wyr4(off(p, 0x00))^key^_wyp0, _wyr4(off(p, 0))^key^_wyp1)^key, 8^_wyp4)
}

func _wx8(key uint8) uint64 { // 1 byte
	p := ptr(&key)

	key64 := uint64(key)

	return _wmum(_wmum(_wyr1(p)^key64^_wyp0, key64^_wyp1)^key64, 1^_wyp4)
}

func _wx16(key uint16) uint64 { // 2 bytes
	p := ptr(&key)

	key64 := uint64(key)

	return _wmum(_wmum(_wyr1(off(p, 0x00))^key64^_wyp0, _wyr1(off(p, 0x00))^key64^_wyp1)^key64, 2^_wyp4)
}

func _wx32(key uint32) uint64 { // 4 byte
	p := ptr(&key)

	key64 := uint64(key)

	return _wmum(_wmum(_wyr2(off(p, 0x00))^key64^_wyp0, _wyr2(off(p, 0x00))^key64^_wyp1)^key64, 4^_wyp4)

}

//go:nocheckptr
func _wyr4(p ptr) uint64 {
	// b := ()(p)

	v := *(*[4]byte)(p)

	// v = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])

	return uint64(uint32(v[0]) | uint32(v[1])<<8 | uint32(v[2])<<16 | uint32(v[3])<<24)
}

//go:nocheckptr
func _wyr2(p ptr) uint64 {
	b := (*[2]byte)(p)
	return uint64(uint16(b[0]) | uint16(b[1])<<8)
}

//go:nocheckptr
func _wyr1(p ptr) uint64 {
	return uint64(*(*byte)(p))
}

//go:nocheckptr
func _wyr3(p ptr, k uintptr) uint64 {
	b0 := uint64(*(*byte)(p))
	b1 := uint64(*(*byte)(off(p, k>>1)))
	b2 := uint64(*(*byte)(off(p, k-1)))
	return b0<<16 | b1<<8 | b2
}

//go:nocheckptr
func _wyr8(p ptr) uint64 {
	b := (*[8]byte)(p)
	return uint64(uint32(b[0])|uint32(b[1])<<8|uint32(b[2])<<16|uint32(b[3])<<24)<<32 |
		uint64(uint32(b[4])|uint32(b[5])<<8|uint32(b[6])<<16|uint32(b[7])<<24)
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
	return uint64(readU32(p, o)) | uint64(readU32(p, o+4))<<32
}

func read64_m(u uint64) uint64 {
	return bits.RotateLeft64(u, 31)

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

func AvalancheSmall(x uint64) uint64 {
	x ^= x >> 33
	x *= prime2
	x ^= x >> 29
	x *= prime3
	x ^= x >> 32
	return x
}

func AvalancheFull(x uint64) uint64 {
	x ^= x >> 33
	x *= prime2
	x ^= x >> 29
	x *= prime3
	x ^= x >> 32
	return x
}

func Avalanche(x uint64) uint64 {
	x ^= x >> 37
	x *= 0x165667919e3779f9
	x ^= x >> 32
	return x
}

func _wmum(x, y uint64) uint64 {

	hi, lo := bits.Mul64(x, y)
	return hi ^ lo
}

func _wyrot(x uint64) uint64 {

	return (x >> 32) | (x << 32)
}
