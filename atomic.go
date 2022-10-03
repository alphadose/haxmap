package haxmap

import (
	"sync/atomic"
	"unsafe"
)

// noCopy implements sync.Locker so that go vet can trigger
// warnings when types embedding noCopy are copied.
type noCopy struct{}

func (c *noCopy) Lock()   {}
func (c *noCopy) Unlock() {}

type atomicUint32 struct {
	_ noCopy
	v uint32
}

type atomicPointer[T any] struct {
	_   noCopy
	ptr unsafe.Pointer
}

type atomicUintptr struct {
	_   noCopy
	ptr uintptr
}

func (u *atomicUint32) Load() uint32            { return atomic.LoadUint32(&u.v) }
func (u *atomicUint32) Store(v uint32)          { atomic.StoreUint32(&u.v, v) }
func (u *atomicUint32) Add(delta uint32) uint32 { return atomic.AddUint32(&u.v, delta) }
func (u *atomicUint32) Swap(v uint32) uint32    { return atomic.SwapUint32(&u.v, v) }
func (u *atomicUint32) CompareAndSwap(old, new uint32) bool {
	return atomic.CompareAndSwapUint32(&u.v, old, new)
}

func (p *atomicPointer[T]) Load() *T     { return (*T)(atomic.LoadPointer(&p.ptr)) }
func (p *atomicPointer[T]) Store(v *T)   { atomic.StorePointer(&p.ptr, unsafe.Pointer(v)) }
func (p *atomicPointer[T]) Swap(v *T) *T { return (*T)(atomic.SwapPointer(&p.ptr, unsafe.Pointer(v))) }
func (p *atomicPointer[T]) CompareAndSwap(old, new *T) bool {
	return atomic.CompareAndSwapPointer(&p.ptr, unsafe.Pointer(old), unsafe.Pointer(new))
}

func (u *atomicUintptr) Load() uintptr             { return atomic.LoadUintptr(&u.ptr) }
func (u *atomicUintptr) Store(v uintptr)           { atomic.StoreUintptr(&u.ptr, v) }
func (u *atomicUintptr) Add(delta uintptr) uintptr { return atomic.AddUintptr(&u.ptr, delta) }
func (u *atomicUintptr) Swap(v uintptr) uintptr    { return atomic.SwapUintptr(&u.ptr, v) }
func (u *atomicUintptr) CompareAndSwap(old, new uintptr) bool {
	return atomic.CompareAndSwapUintptr(&u.ptr, old, new)
}
