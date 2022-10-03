package haxmap

import (
	"reflect"
	"sort"
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

	// metadata of the hashmap
	metadata[K hashable, V any] struct {
		keyshifts uintptr        //  array_size - log2(array_size)
		count     atomicUintptr  // number of filled items
		data      unsafe.Pointer // pointer to array of map indexes

		// use a struct element with generic params to enable monomorphization (generic code copy-paste) for the parent metadata struct by golang compiler leading to best performance (truly hax)
		// else in other cases the generic params will be unnecessarily passed as function parameters everytime instead of monomorphization leading to slower performance
		index []*element[K, V]
	}

	// Map implements the concurrent hashmap
	Map[K hashable, V any] struct {
		listHead *element[K, V] // Harris lock-free list of elements in ascending order of hash
		hasher   func(K) uintptr
		metadata atomicPointer[metadata[K, V]] // atomic.Pointer for safe access even during resizing
		resizing atomicUint32
		numItems atomicUintptr
	}

	// used in deletion of map elements
	deletionRequest[K hashable] struct {
		keyHash uintptr
		key     K
	}
)

// New returns a new HashMap instance with an optional specific initialization size
func New[K hashable, V any](size ...uintptr) *Map[K, V] {
	m := &Map[K, V]{listHead: newListHead[K, V]()}
	m.numItems.Store(0)
	if len(size) > 0 {
		m.allocate(size[0])
	} else {
		m.allocate(defaultSize)
	}
	m.setDefaultHasher()
	return m
}

// Del deletes key/keys from the map
// Bulk deletion is more efficient than deleting keys one by one
func (m *Map[K, V]) Del(keys ...K) {
	if len(keys) == 0 {
		return
	}
	var (
		size = len(keys)
		delQ = make([]deletionRequest[K], size)
		elem = m.listHead.next()
		iter = 0
	)
	for idx := 0; idx < size; idx++ {
		delQ[idx].keyHash, delQ[idx].key = m.hasher(keys[idx]), keys[idx]
	}

	// sort in ascending order of keyhash
	sort.Slice(delQ, func(i, j int) bool {
		return delQ[i].keyHash < delQ[j].keyHash
	})

	for elem != nil && iter < size {
		if elem.keyHash == delQ[iter].keyHash && elem.key == delQ[iter].key {
			elem.remove() // mark node for deletion
			m.removeItemFromIndex(elem)
			iter++
			elem = elem.next()
		} else if elem.keyHash > delQ[iter].keyHash {
			iter++
		} else {
			elem = elem.next()
		}
	}

	// iterate list from start to end to remove the marked nodes
	for iter := m.listHead; iter != nil; iter = iter.next() {
	}
}

// Get retrieves an element from the map
// returns `false“ if element is absent
func (m *Map[K, V]) Get(key K) (value V, ok bool) {
	h := m.hasher(key)
	// inline search
	for elem := m.metadata.Load().indexElement(h); elem != nil; elem = elem.nextPtr.Load() {
		if elem.keyHash == h && elem.key == key {
			value, ok = *elem.value.Load(), true
			return
		}
		if elem.keyHash <= h || elem.keyHash == marked {
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
func (m *Map[K, V]) Set(key K, value V) {
	var (
		h        = m.hasher(key)
		valPtr   = &value
		alloc    *element[K, V]
		created  = false
		data     = m.metadata.Load()
		existing = data.indexElement(h)
	)

	if existing == nil || existing.keyHash > h {
		existing = m.listHead
	}
	if alloc, created = existing.inject(h, key, valPtr); alloc != nil {
		if created {
			m.numItems.Add(1)
		}
	} else {
		for existing = m.listHead; alloc == nil; alloc, created = existing.inject(h, key, valPtr) {
		}
		if created {
			m.numItems.Add(1)
		}
	}

	count := data.addItemToIndex(alloc)
	if resizeNeeded(uintptr(len(data.index)), count) && m.resizing.CompareAndSwap(notResizing, resizingInProgress) {
		m.grow(0) // double in size
	}
}

// ForEach iterates over key-value pairs and executes the lambda provided for each such pair
// lambda must return `true` to continue iteration and `false` to break iteration
func (m *Map[K, V]) ForEach(lambda func(K, V) bool) {
	for item := m.listHead.next(); item != nil && lambda(item.key, *item.value.Load()); item = item.next() {
	}
}

// Grow resizes the hashmap to a new size, gets rounded up to next power of 2
// To double the size of the hashmap use newSize 0
// No resizing is done in case of another resize operation already being in progress
// Growth and map bucket policy is inspired from https://github.com/cornelk/hashmap
func (m *Map[K, V]) Grow(newSize uintptr) {
	if m.resizing.CompareAndSwap(notResizing, resizingInProgress) {
		m.grow(newSize)
	}
}

// SetHasher sets the hash function to the one provided by the user
func (m *Map[K, V]) SetHasher(hs func(K) uintptr) {
	m.hasher = hs
}

// Len returns the number of key-value pairs within the map
func (m *Map[K, V]) Len() uintptr {
	return m.numItems.Load()
}

// Fillrate returns the fill rate of the map as an percentage integer
func (m *Map[K, V]) Fillrate() uintptr {
	data := m.metadata.Load()
	return (data.count.Load() * 100) / uintptr(len(data.index))
}

// allocate map with the given size
func (m *Map[K, V]) allocate(newSize uintptr) {
	if m.resizing.CompareAndSwap(notResizing, resizingInProgress) {
		m.grow(newSize)
	}
}

// fillIndexItems re-indexes the map given the latest state of the linked list
func (m *Map[K, V]) fillIndexItems(mapData *metadata[K, V]) {
	first := m.listHead
	item := first
	lastIndex := uintptr(0)
	for item != nil {
		index := item.keyHash >> mapData.keyshifts
		if item == first || index != lastIndex {
			mapData.addItemToIndex(item)
			lastIndex = index
		}
		item = item.next()
	}
}

// removeItemFromIndex removes an item from the map index
func (m *Map[K, V]) removeItemFromIndex(item *element[K, V]) {
	for {
		data := m.metadata.Load()
		index := item.keyHash >> data.keyshifts
		ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(data.data) + index*intSizeBytes))

		next := item.next()
		if next != nil && item.keyHash>>data.keyshifts != index {
			next = nil // do not set index to next item if it's not the same slice index
		}
		atomic.CompareAndSwapPointer(ptr, unsafe.Pointer(item), unsafe.Pointer(next))

		if data == m.metadata.Load() { // check that no resize happened
			m.numItems.Add(marked)
			return
		}
	}
}

// grow to the new size
func (m *Map[K, V]) grow(newSize uintptr) {
	for {
		currentStore := m.metadata.Load()
		if newSize == 0 {
			newSize = uintptr(len(currentStore.index)) << 1
		} else {
			newSize = roundUpPower2(newSize)
		}

		index := make([]*element[K, V], newSize)
		header := (*reflect.SliceHeader)(unsafe.Pointer(&index))

		newdata := &metadata[K, V]{
			keyshifts: strconv.IntSize - log2(newSize),
			data:      unsafe.Pointer(header.Data),
			index:     index,
		}

		m.fillIndexItems(newdata) // re-index with longer and more widespread keys
		m.metadata.Store(newdata)

		if !resizeNeeded(newSize, uintptr(m.Len())) {
			m.resizing.Store(notResizing)
			return
		}
		newSize = 0 // 0 means double the current size
	}
}

// indexElement returns the index of a hash key, returns `nil` if absent
func (md *metadata[K, V]) indexElement(hashedKey uintptr) *element[K, V] {
	index := hashedKey >> md.keyshifts
	ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(md.data) + index*intSizeBytes))
	item := (*element[K, V])(atomic.LoadPointer(ptr))
	for (item == nil || hashedKey < item.keyHash) && index > 0 {
		index--
		ptr = (*unsafe.Pointer)(unsafe.Pointer(uintptr(md.data) + index*intSizeBytes))
		item = (*element[K, V])(atomic.LoadPointer(ptr))
	}
	return item
}

// addItemToIndex adds an item to the index if needed and returns the new item counter if it changed, otherwise 0
func (md *metadata[K, V]) addItemToIndex(item *element[K, V]) uintptr {
	index := item.keyHash >> md.keyshifts
	ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(md.data) + index*intSizeBytes))
	for {
		elem := (*element[K, V])(atomic.LoadPointer(ptr))
		if elem == nil {
			if atomic.CompareAndSwapPointer(ptr, nil, unsafe.Pointer(item)) {
				return md.count.Add(1)
			}
			continue
		}

		if item.keyHash < elem.keyHash {
			if !atomic.CompareAndSwapPointer(ptr, unsafe.Pointer(elem), unsafe.Pointer(item)) {
				continue
			}
		}
		return 0
	}
}

// check if resize is needed
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
func log2(i uintptr) (n uintptr) {
	for p := uintptr(1); p < i; p, n = p<<1, n+1 {
	}
	return
}
