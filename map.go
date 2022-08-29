package haxmap

import (
	"reflect"
	"strconv"
	"sync/atomic"
	"unsafe"
)

const (
	// hash input allowed sizes
	byteSize = 1 << iota
	wordSize
	dwordSize
	qwordSize
	owordSize

	// DefaultSize is the default size for a zero allocated map
	DefaultSize = 256

	// MaxFillRate is the maximum fill rate for the slice before a resize  will happen.
	MaxFillRate = 50

	// intSizeBytes is the size in byte of an int or uint value.
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
		Keyshifts uintptr        // Pointer size - log2 of array size, to be used as index in the data array
		count     atomic.Uintptr // count of filled elements in the slice
		data      unsafe.Pointer // pointer to slice data array
		index     []*element[K, V]
	}

	// HashMap implements a read optimized hash map.
	HashMap[K hashable, V any] struct {
		listHead *element[K, V] // key sorted linked list of elements
		hasher   func(K) uintptr
		Datamap  atomic.Pointer[hashMapData[K, V]] // pointer to a map instance that gets replaced if the map resizes
		resizing atomic.Uint32
		numItems atomic.Uintptr
	}
)

// New returns a new HashMap instance with an optional specific initialization size.
func New[K hashable, V any](size ...uintptr) *HashMap[K, V] {
	m := &HashMap[K, V]{listHead: newListHead[K, V]()}
	m.numItems.Store(0)
	if len(size) > 0 {
		m.allocate(size[0])
	} else {
		m.allocate(DefaultSize)
	}
	m.setDefaultHasher()
	return m
}

// returns the index of a hash key, returns `nil` if absent
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

// Del deletes the key from the map.
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

// Get retrieves an element from the map under given hash key.
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

// Set sets the value under the specified key to the map. An existing item for this key will be overwritten.
// If a resizing operation is happening concurrently while calling Set, the item might show up in the map only after the resize operation is finished.
func (m *HashMap[K, V]) Set(key K, value V) {
	h, valPtr := m.hasher(key), &value
	var (
		alloc   *element[K, V]
		created = false
	)
	for {
		data := m.Datamap.Load()
		if data == nil {
			m.Grow(DefaultSize)
			continue // read mapdata and slice item again
		}
		existing := data.indexElement(h)
		if existing == nil {
			existing = m.listHead
		}
		if alloc, created = existing.inject(h, key, valPtr); created {
			m.numItems.Add(1)
		}

		count := data.addItemToIndex(alloc)
		if resizeNeeded(uintptr(len(data.index)), count) && m.resizing.CompareAndSwap(notResizing, resizingInProgress) {
			go m.grow(0, true)
		}
		return
	}
}

// adds an item to the index if needed and returns the new item counter if it changed, otherwise 0
func (mapData *hashMapData[K, V]) addItemToIndex(item *element[K, V]) uintptr {
	index := item.keyHash >> mapData.Keyshifts
	ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(mapData.data) + index*intSizeBytes))

	for { // loop until the smallest key hash is in the index
		element := (*element[K, V])(atomic.LoadPointer(ptr)) // get the current item in the index
		if element == nil {                                  // no item yet at this index
			if atomic.CompareAndSwapPointer(ptr, nil, unsafe.Pointer(item)) {
				return mapData.count.Add(1)
			}
			continue // a new item was inserted concurrently, retry
		}

		if item.keyHash < element.keyHash {
			// the new item is the smallest for this index?
			if !atomic.CompareAndSwapPointer(ptr, unsafe.Pointer(element), unsafe.Pointer(item)) {
				continue // a new item was inserted concurrently, retry
			}
		}
		return 0
	}
}

func (m *HashMap[K, V]) fillIndexItems(mapData *hashMapData[K, V]) {
	first := m.listHead
	item := first
	lastIndex := uintptr(0)

	for item != nil {
		index := item.keyHash >> mapData.Keyshifts
		if item == first || index != lastIndex { // store item with smallest hash key for every index
			mapData.addItemToIndex(item)
			lastIndex = index
		}
		item = item.next()
	}
}

// ForEach iterates over key-value pairs and executes the lambda provided for each such pair.
func (m *HashMap[K, V]) ForEach(lambda func(K, V)) {
	for item := m.listHead.next(); item != nil; item = item.next() {
		lambda(item.key, *item.value.Load())
	}
}

// Grow resizes the hashmap to a new size, gets rounded up to next power of 2.
// To double the size of the hashmap use newSize 0.
// This function returns immediately, the resize operation is done in a goroutine.
// No resizing is done in case of another resize operation already being in progress.
func (m *HashMap[K, V]) Grow(newSize uintptr) {
	if m.resizing.CompareAndSwap(notResizing, resizingInProgress) {
		m.grow(newSize, true)
	}
}

// SetHasher sets the hash function to the one provided by the user
func (m *HashMap[K, V]) SetHasher(hs func(K) uintptr) {
	m.hasher = hs
}

// Len returns the number of key-value pairs within the map.
func (m *HashMap[K, V]) Len() uintptr {
	return m.numItems.Load()
}

// Fillrate returns the fill rate of the map as an percentage integer.
func (m *HashMap[K, V]) Fillrate() uintptr {
	data := m.Datamap.Load()
	return (data.count.Load() * 100) / uintptr(len(data.index))
}

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
			data:      unsafe.Pointer(header.Data), // use address of slice data storage
			index:     index,
		}

		m.fillIndexItems(newdata) // initialize new index slice with longer keys

		m.Datamap.Store(newdata)

		m.fillIndexItems(newdata) // make sure that the new index is up-to-date with the current state of the linked list

		if !loop {
			return
		}

		// check if a new resize needs to be done already
		if !resizeNeeded(newSize, uintptr(m.Len())) {
			return
		}
		newSize = 0 // 0 means double the current size
	}
}

func resizeNeeded(length, count uintptr) bool {
	return (count*100)/length > MaxFillRate
}

// roundUpPower2 rounds a number to the next power of 2.
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

// log2 computes the binary logarithm of x, rounded up to the next integer.
func log2(i uintptr) uintptr {
	var n, p uintptr
	for p = 1; p < i; p += p {
		n++
	}
	return n
}
