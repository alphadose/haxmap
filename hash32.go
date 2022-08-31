//go:build 386 || arm || mips || mipsle

package haxmap

/*
From https://github.com/pierrec/xxHash

Copyright (c) 2014, Pierre Curto
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

* Neither the name of xxHash nor the names of its
  contributors may be used to endorse or promote products derived from
  this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

import (
	"encoding/binary"
	"math/bits"
	"reflect"
	"unsafe"
)

const (
	prime1 uint32 = 2654435761
	prime2 uint32 = 2246822519
	prime3 uint32 = 3266489917
	prime4 uint32 = 668265263
	prime5 uint32 = 374761393
)

var prime1v = prime1

// defaultSum implements xxHash for 32 bit systems
func defaultSum(b []byte) uintptr {
	n := len(b)
	h32 := uint32(n)

	if n < 16 {
		h32 += prime5
	} else {
		v1 := prime1v + prime2
		v2 := prime2
		v3 := uint32(0)
		v4 := -prime1v
		p := 0
		for n := n - 16; p <= n; p += 16 {
			sub := b[p:][:16] //BCE hint for compiler
			v1 = rol13(v1+u32(sub[:])*prime2) * prime1
			v2 = rol13(v2+u32(sub[4:])*prime2) * prime1
			v3 = rol13(v3+u32(sub[8:])*prime2) * prime1
			v4 = rol13(v4+u32(sub[12:])*prime2) * prime1
		}
		b = b[p:]
		n -= p
		h32 += rol1(v1) + rol7(v2) + rol12(v3) + rol18(v4)
	}

	p := 0
	for n := n - 4; p <= n; p += 4 {
		h32 += u32(b[p:p+4]) * prime3
		h32 = rol17(h32) * prime4
	}
	for p < n {
		h32 += uint32(b[p]) * prime5
		h32 = rol11(h32) * prime1
		p++
	}

	h32 ^= h32 >> 15
	h32 *= prime2
	h32 ^= h32 >> 13
	h32 *= prime3
	h32 ^= h32 >> 16

	return uintptr(h32)
}

func u32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

func rol1(x uint32) uint32  { return bits.RotateLeft32(x, 1) }
func rol7(x uint32) uint32  { return bits.RotateLeft32(x, 7) }
func rol11(x uint32) uint32 { return bits.RotateLeft32(x, 11) }
func rol12(x uint32) uint32 { return bits.RotateLeft32(x, 12) }
func rol13(x uint32) uint32 { return bits.RotateLeft32(x, 13) }
func rol17(x uint32) uint32 { return bits.RotateLeft32(x, 17) }
func rol18(x uint32) uint32 { return bits.RotateLeft32(x, 18) }

func (m *HashMap[K, V]) setDefaultHasher() {
	// default hash functions
	switch any(*new(K)).(type) {
	case int, uint, uintptr:
		m.hasher = func(key K) uintptr {
			return defaultSum(unsafe.Slice((*byte)(unsafe.Pointer(&key)), intSizeBytes))
		}
	case int8, uint8:
		m.hasher = func(key K) uintptr {
			return defaultSum(unsafe.Slice((*byte)(unsafe.Pointer(&key)), byteSize))
		}
	case int16, uint16:
		m.hasher = func(key K) uintptr {
			return defaultSum(unsafe.Slice((*byte)(unsafe.Pointer(&key)), wordSize))
		}
	case int32, uint32, float32:
		m.hasher = func(key K) uintptr {
			return defaultSum(unsafe.Slice((*byte)(unsafe.Pointer(&key)), dwordSize))
		}
	case int64, uint64, float64, complex64:
		m.hasher = func(key K) uintptr {
			return defaultSum(unsafe.Slice((*byte)(unsafe.Pointer(&key)), qwordSize))
		}
	case complex128:
		m.hasher = func(key K) uintptr {
			return defaultSum(unsafe.Slice((*byte)(unsafe.Pointer(&key)), owordSize))
		}
	case string:
		m.hasher = func(key K) uintptr {
			sh := (*reflect.StringHeader)(unsafe.Pointer(&key))
			return defaultSum(unsafe.Slice((*byte)(unsafe.Pointer(&key)), sh.Len))
		}
	}
}
