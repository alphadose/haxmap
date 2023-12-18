package haxmap

import (
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"sync/atomic"
	"unsafe"

	"golang.org/x/exp/constraints"
)

const (
	// defaultSize is the default size for a zero allocated map
	defaultSize = 8

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
	hashable interface {
		constraints.Integer | constraints.Float | constraints.Complex | ~string | uintptr | ~unsafe.Pointer
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
	if len(size) > 0 && size[0] != 0 {
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
	size := len(keys)
	switch {
	case size == 0:
		return
	case size == 1: // delete one
		var (
			h        = m.hasher(keys[0])
			existing = m.metadata.Load().indexElement(h)
		)
		if existing == nil || existing.keyHash > h {
			existing = m.listHead.next()
		}
		for ; existing != nil && existing.keyHash <= h; existing = existing.next() {
			if existing.key == keys[0] {
				if existing.remove() { // mark node for lazy removal on next pass
					m.removeItemFromIndex(existing) // remove node from map index
				}
				return
			}
		}
	default: // delete multiple entries
		var (
			delQ = make([]deletionRequest[K], size)
			iter = 0
		)
		for idx := 0; idx < size; idx++ {
			delQ[idx].keyHash, delQ[idx].key = m.hasher(keys[idx]), keys[idx]
		}

		// sort in ascending order of keyhash
		sort.Slice(delQ, func(i, j int) bool {
			return delQ[i].keyHash < delQ[j].keyHash
		})

		elem := m.metadata.Load().indexElement(delQ[0].keyHash)

		if elem == nil || elem.keyHash > delQ[0].keyHash {
			elem = m.listHead.next()
		}

		for elem != nil && iter < size {
			if elem.keyHash == delQ[iter].keyHash && elem.key == delQ[iter].key {
				if elem.remove() { // mark node for lazy removal on next pass
					m.removeItemFromIndex(elem) // remove node from map index
				}
				iter++
				elem = elem.next()
			} else if elem.keyHash > delQ[iter].keyHash {
				iter++
			} else {
				elem = elem.next()
			}
		}
	}
}

// Get retrieves an element from the map
// returns `false“ if element is absent
func (m *Map[K, V]) Get(key K) (value V, ok bool) {
	h := m.hasher(key)
	// inline search
	for elem := m.metadata.Load().indexElement(h); elem != nil && elem.keyHash <= h; elem = elem.nextPtr.Load() {
		if elem.key == key {
			value, ok = *elem.value.Load(), !elem.isDeleted()
			return
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

// GetOrSet returns the existing value for the key if present
// Otherwise, it stores and returns the given value
// The loaded result is true if the value was loaded, false if stored
func (m *Map[K, V]) GetOrSet(key K, value V) (actual V, loaded bool) {
	var (
		h        = m.hasher(key)
		data     = m.metadata.Load()
		existing = data.indexElement(h)
	)
	// try to get the element if present
	for elem := existing; elem != nil && elem.keyHash <= h; elem = elem.nextPtr.Load() {
		if elem.key == key && !elem.isDeleted() {
			actual, loaded = *elem.value.Load(), true
			return
		}
	}
	// Get() failed because element is absent
	// store the value given by user
	actual, loaded = value, false

	var (
		alloc   *element[K, V]
		created = false
		valPtr  = &value
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
	return
}

// GetOrCompute is similar to GetOrSet but the value to be set is obtained from a constructor
// the value constructor is called only once
func (m *Map[K, V]) GetOrCompute(key K, valueFn func() V) (actual V, loaded bool) {
	var (
		h        = m.hasher(key)
		data     = m.metadata.Load()
		existing = data.indexElement(h)
	)
	// try to get the element if present
	for elem := existing; elem != nil && elem.keyHash <= h; elem = elem.nextPtr.Load() {
		if elem.key == key && !elem.isDeleted() {
			actual, loaded = *elem.value.Load(), true
			return
		}
	}
	// Get() failed because element is absent
	// compute the value from the constructor and store it
	value := valueFn()
	actual, loaded = value, false

	var (
		alloc   *element[K, V]
		created = false
		valPtr  = &value
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
	return
}

// GetAndDel deletes the key from the map, returning the previous value if any.
func (m *Map[K, V]) GetAndDel(key K) (value V, ok bool) {
	var (
		h        = m.hasher(key)
		existing = m.metadata.Load().indexElement(h)
	)
	if existing == nil || existing.keyHash > h {
		existing = m.listHead.next()
	}
	for ; existing != nil && existing.keyHash <= h; existing = existing.next() {
		if existing.key == key {
			value, ok = *existing.value.Load(), !existing.isDeleted()
			if existing.remove() {
				m.removeItemFromIndex(existing)
			}
			return
		}
	}
	return
}

// CompareAndSwap atomically updates a map entry given its key by comparing current value to `oldValue`
// and setting it to `newValue` if the above comparison is successful
// It returns a boolean indicating whether the CompareAndSwap was successful or not
func (m *Map[K, V]) CompareAndSwap(key K, oldValue, newValue V) bool {
	var (
		h        = m.hasher(key)
		existing = m.metadata.Load().indexElement(h)
	)
	if existing == nil || existing.keyHash > h {
		existing = m.listHead
	}
	if _, current, _ := existing.search(h, key); current != nil {
		if oldPtr := current.value.Load(); reflect.DeepEqual(*oldPtr, oldValue) {
			return current.value.CompareAndSwap(oldPtr, &newValue)
		}
	}
	return false
}

// Swap atomically swaps the value of a map entry given its key
// It returns the old value if swap was successful and a boolean `swapped` indicating whether the swap was successful or not
func (m *Map[K, V]) Swap(key K, newValue V) (oldValue V, swapped bool) {
	var (
		h        = m.hasher(key)
		existing = m.metadata.Load().indexElement(h)
	)
	if existing == nil || existing.keyHash > h {
		existing = m.listHead
	}
	if _, current, _ := existing.search(h, key); current != nil {
		oldValue, swapped = *current.value.Swap(&newValue), true
	} else {
		swapped = false
	}
	return
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

// MarshalJSON implements the json.Marshaler interface.
func (m *Map[K, V]) MarshalJSON() ([]byte, error) {
	gomap := make(map[K]V)
	for i := m.listHead.next(); i != nil; i = i.next() {
		gomap[i.key] = *i.value.Load()
	}
	return json.Marshal(gomap)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (m *Map[K, V]) UnmarshalJSON(i []byte) error {
	gomap := make(map[K]V)
	err := json.Unmarshal(i, &gomap)
	if err != nil {
		return err
	}
	for k, v := range gomap {
		m.Set(k, v)
	}
	return nil
}

// allocate map with the given size
func (m *Map[K, V]) allocate(newSize uintptr) {
	if m.resizing.CompareAndSwap(notResizing, resizingInProgress) {
		m.grow(newSize)
	}
}

// fillIndexItems re-indexes the map given the latest state of the linked list
func (m *Map[K, V]) fillIndexItems(mapData *metadata[K, V]) {
	var (
		first     = m.listHead.next()
		item      = first
		lastIndex = uintptr(0)
	)
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
		if next != nil && next.keyHash>>data.keyshifts != index {
			next = nil // do not set index to next item if it's not the same slice index
		}
		swappedToNil := atomic.CompareAndSwapPointer(ptr, unsafe.Pointer(item), unsafe.Pointer(next)) && next == nil

		if data == m.metadata.Load() { // check that no resize happened
			m.numItems.Add(^uintptr(0)) // decrement counter
			if swappedToNil {           // decrement the metadata count if the index is set to nil
				data.count.Add(^uintptr(0))
			}
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
	for (item == nil || hashedKey < item.keyHash || item.isDeleted()) && index > 0 {
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
