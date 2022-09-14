package haxmap

import (
	"reflect"
	"strconv"
	"sync/atomic"
	"unsafe"
)

const (
	// defaultSize is the default size for a zero allocated map
	defaultSize = 256

	// maxFillRate is the maximum fill rate for the slice before a resize will happen
	maxFillRate = 50

	// intSizeBytes is the size in byte of an int or uint value
	intSizeBytes = strconv.IntSize >> 3
)

// indicates resizing operation status enums
const (
	notResizing uint32 = iota
	resizingInProgress
)

type (
	// allowed map key types constraint
	hashable interface {
		int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | uintptr | float32 | float64 | string | complex64 | complex128
	}

	hashMapData[K hashable, V any] struct {
		Keyshifts uintptr        //  array_size - log2(array_size)
		count     atomic.Uintptr // number of filled items
		data      unsafe.Pointer // pointer to array
		index     []*element[K, V]
	}

	// HashMap implements the concurrent hashmap
	HashMap[K hashable, V any] struct {
		listHead *element[K, V] // Harris lock-free list of elements in ascending order of hash
		hasher   func(K) uintptr
		Datamap  atomic.Pointer[hashMapData[K, V]] // atomic.Pointer for safe access even during resizing
		resizing atomic.Uint32
		numItems atomic.Uintptr
	}
)

// New returns a new HashMap instance with an optional specific initialization size
func New[K hashable, V any](size ...uintptr) *HashMap[K, V] {
	m := &HashMap[K, V]{listHead: newListHead[K, V]()}
	m.numItems.Store(0)
	if len(size) > 0 {
		m.allocate(size[0])
	} else {
		m.allocate(defaultSize)
	}
	m.setDefaultHasher()
	return m
}

// indexElement returns the index of a hash key, returns `nil` if absent
func (mapData *hashMapData[K, V]) indexElement(hashedKey uintptr) *element[K, V] {
	index := hashedKey >> mapData.Keyshifts
	ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(mapData.data) + index*intSizeBytes))
	item := (*element[K, V])(atomic.LoadPointer(ptr))
	for (item == nil || hashedKey < item.keyHash) && index > 0 {
		index--
		ptr = (*unsafe.Pointer)(unsafe.Pointer(uintptr(mapData.data) + index*intSizeBytes))
		item = (*element[K, V])(atomic.LoadPointer(ptr))
	}
	return item
}

// Del deletes the key from the map
// does nothing if key is absemt
func (m *HashMap[K, V]) Del(key K) {
	h := m.hasher(key)

	element := m.Datamap.Load().indexElement(h)
loop:
	for ; element != nil; element = element.next() {
		if element.keyHash == h && element.key == key {
			break loop
		}
		if element.keyHash > h {
			return
		}
	}
	if element == nil {
		return
	}
	element.remove()
	// ensure complete deletion via iterating the list
	for iter := m.listHead; iter != nil; iter = iter.next() {
	}
	for {
		data := m.Datamap.Load()
		index := element.keyHash >> data.Keyshifts
		ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(data.data) + index*intSizeBytes))

		next := element.next()
		if next != nil && element.keyHash>>data.Keyshifts != index {
			next = nil // do not set index to next item if it's not the same slice index
		}
		atomic.CompareAndSwapPointer(ptr, unsafe.Pointer(element), unsafe.Pointer(next))

		if data == m.Datamap.Load() { // check that no resize happened
			m.numItems.Add(marked)
			return
		}
	}
}

// Get retrieves an element from the map
// returns `false“ if element is absent
func (m *HashMap[K, V]) Get(key K) (value V, ok bool) {
	h := m.hasher(key)
	// inline search
	for elem := m.Datamap.Load().indexElement(h); elem != nil; elem = elem.nextPtr.Load() {
		if elem.keyHash == h && elem.key == key {
			value, ok = *elem.value.Load(), true
			return
		}
		if elem.keyHash == marked || elem.keyHash <= h {
			continue
		} else {
			break
		}
	}
	ok = false
	return
}

// Set tries to update an element if key is present else it inserts a new element
// If a resizing operation is happening concurrently while calling Set()
// then the item might show up in the map only after the resize operation is finished
func (m *HashMap[K, V]) Set(key K, value V) {
	h, valPtr := m.hasher(key), &value
	var (
		alloc   *element[K, V]
		created = false
	)

start:
	data := m.Datamap.Load()
	if data == nil {
		m.Grow(defaultSize)
		goto start // read mapdata and slice item again
	}
	existing := data.indexElement(h)
	if existing == nil || existing.keyHash > h {
		existing = m.listHead
	}
	if alloc, created = existing.inject(h, key, valPtr); created {
		m.numItems.Add(1)
	}

	count := data.addItemToIndex(alloc)
	if resizeNeeded(uintptr(len(data.index)), count) && m.resizing.CompareAndSwap(notResizing, resizingInProgress) {
		m.grow(0, true)
	}
}

// addItemToIndex adds an item to the index if needed and returns the new item counter if it changed, otherwise 0
func (mapData *hashMapData[K, V]) addItemToIndex(item *element[K, V]) uintptr {
	index := item.keyHash >> mapData.Keyshifts
	ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(mapData.data) + index*intSizeBytes))
	for {
		element := (*element[K, V])(atomic.LoadPointer(ptr))
		if element == nil {
			if atomic.CompareAndSwapPointer(ptr, nil, unsafe.Pointer(item)) {
				return mapData.count.Add(1)
			}
			continue
		}

		if item.keyHash < element.keyHash {
			if !atomic.CompareAndSwapPointer(ptr, unsafe.Pointer(element), unsafe.Pointer(item)) {
				continue
			}
		}
		return 0
	}
}

// fillIndexItems re-indexes the map given the latest state of the linked list
func (m *HashMap[K, V]) fillIndexItems(mapData *hashMapData[K, V]) {
	first := m.listHead
	item := first
	lastIndex := uintptr(0)
	for item != nil {
		index := item.keyHash >> mapData.Keyshifts
		if item == first || index != lastIndex {
			mapData.addItemToIndex(item)
			lastIndex = index
		}
		item = item.next()
	}
}

// ForEach iterates over key-value pairs and executes the lambda provided for each such pair
// lambda must return `true` to continue iteration and `false` to break iteration
func (m *HashMap[K, V]) ForEach(lambda func(K, V) bool) {
	for item := m.listHead.nextPtr.Load(); item != nil; item = item.nextPtr.Load() {
		if item.keyHash == marked {
			continue
		}
		if !lambda(item.key, *item.value.Load()) {
			return
		}
	}
}

// Grow resizes the hashmap to a new size, gets rounded up to next power of 2
// To double the size of the hashmap use newSize 0
// This function returns immediately, the resize operation is done in a goroutine
// No resizing is done in case of another resize operation already being in progress
// Growth and map bucket policy is inspired from https://github.com/cornelk/hashmap
func (m *HashMap[K, V]) Grow(newSize uintptr) {
	if m.resizing.CompareAndSwap(notResizing, resizingInProgress) {
		m.grow(newSize, true)
	}
}

// SetHasher sets the hash function to the one provided by the user
func (m *HashMap[K, V]) SetHasher(hs func(K) uintptr) {
	m.hasher = hs
}

// Len returns the number of key-value pairs within the map
func (m *HashMap[K, V]) Len() uintptr {
	return m.numItems.Load()
}

// Fillrate returns the fill rate of the map as an percentage integer
func (m *HashMap[K, V]) Fillrate() uintptr {
	data := m.Datamap.Load()
	return (data.count.Load() * 100) / uintptr(len(data.index))
}

// allocate map with the given size
func (m *HashMap[K, V]) allocate(newSize uintptr) {
	if m.resizing.CompareAndSwap(notResizing, resizingInProgress) {
		m.grow(newSize, false)
	}
}

// grow to the new size
func (m *HashMap[K, V]) grow(newSize uintptr, loop bool) {
	defer m.resizing.CompareAndSwap(resizingInProgress, notResizing)

	for {
		currentStore := m.Datamap.Load()
		if newSize == 0 {
			newSize = uintptr(len(currentStore.index)) << 1
		} else {
			newSize = roundUpPower2(newSize)
		}

		index := make([]*element[K, V], newSize, newSize)
		header := (*reflect.SliceHeader)(unsafe.Pointer(&index))

		newdata := &hashMapData[K, V]{
			Keyshifts: strconv.IntSize - log2(newSize),
			data:      unsafe.Pointer(header.Data),
			index:     index,
		}

		m.fillIndexItems(newdata) // re-index with longer and more widespread keys

		m.Datamap.Store(newdata)

		if !loop {
			return
		}

		if !resizeNeeded(newSize, uintptr(m.Len())) {
			return
		}
		newSize = 0 // 0 means double the current size
	}
}

func resizeNeeded(length, count uintptr) bool {
	return (count*100)/length > maxFillRate
}

// roundUpPower2 rounds a number to the next power of 2
func roundUpPower2(i uintptr) uintptr {
	i--
	i |= i >> 1
	i |= i >> 2
	i |= i >> 4
	i |= i >> 8
	i |= i >> 16
	i |= i >> 32
	i++
	return i
}

// log2 computes the binary logarithm of x, rounded up to the next integer
func log2(i uintptr) uintptr {
	var n, p uintptr
	for p = 1; p < i; p += p {
		n++
	}
	return n
}
