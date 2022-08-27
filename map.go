package haxmap

import (
	"reflect"
	"strconv"
	"sync/atomic"
	"unsafe"

	"github.com/alphadose/haxmap/hash"
)

const (
	// hash input allowed sizes
	byteSize = 1 << iota
	wordSize
	dwordSize
	qwordSize
	owordSize

	// DefaultSize is the default size for a zero allocated map
	DefaultSize = 8

	// MaxFillRate is the maximum fill rate for the slice before a resize  will happen.
	MaxFillRate = 50

	// intSizeBytes is the size in byte of an int or uint value.
	intSizeBytes = strconv.IntSize >> 3
)

type (
	// allowed map key types constraint
	hashable interface {
		int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | uintptr | float32 | float64 | string | complex64 | complex128
	}

	hashMapData[K hashable, V any] struct {
		data      unsafe.Pointer // pointer to slice data array
		keyshifts uintptr        // Pointer size - log2 of array size, to be used as index in the data array
		length    uintptr        // current length
		count     atomic.Uintptr // count of filled elements in the slice
	}

	// HashMap implements a read optimized hash map.
	HashMap[K hashable, V any] struct {
		linkedlist *List[K, V] // key sorted linked list of elements
		hasher     func(K) uintptr
		growChan   chan uintptr
		datamap    atomic.Pointer[hashMapData[K, V]] // pointer to a map instance that gets replaced if the map resizes

	}
)

// New returns a new HashMap instance with an optional specific initialization size.
func New[K hashable, V any](size ...uintptr) *HashMap[K, V] {
	m := &HashMap[K, V]{linkedlist: NewList[K, V](), growChan: make(chan uintptr, 3)}
	go m.growRoutine() // asynchronously handle resizing operations
	if len(size) > 0 {
		m.allocate(size[0])
	} else {
		m.allocate(DefaultSize)
	}
	// default hash functions
	switch any(*new(K)).(type) {
	case int, uint, uintptr:
		m.hasher = func(key K) uintptr {
			return hash.Sum(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(unsafe.Pointer(&key)),
				Len:  intSizeBytes,
				Cap:  intSizeBytes,
			})))
		}
	case int8, uint8:
		m.hasher = func(key K) uintptr {
			return hash.Sum(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(unsafe.Pointer(&key)),
				Len:  byteSize,
				Cap:  byteSize,
			})))
		}
	case int16, uint16:
		m.hasher = func(key K) uintptr {
			return hash.Sum(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(unsafe.Pointer(&key)),
				Len:  wordSize,
				Cap:  wordSize,
			})))
		}
	case int32, uint32, float32:
		m.hasher = func(key K) uintptr {
			return hash.Sum(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(unsafe.Pointer(&key)),
				Len:  dwordSize,
				Cap:  dwordSize,
			})))
		}
	case int64, uint64, float64, complex64:
		m.hasher = func(key K) uintptr {
			return hash.Sum(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(unsafe.Pointer(&key)),
				Len:  qwordSize,
				Cap:  qwordSize,
			})))
		}
	case complex128:
		m.hasher = func(key K) uintptr {
			return hash.Sum(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(unsafe.Pointer(&key)),
				Len:  owordSize,
				Cap:  owordSize,
			})))
		}
	case string:
		m.hasher = func(key K) uintptr {
			sh := (*reflect.StringHeader)(unsafe.Pointer(&key))
			return hash.Sum(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: sh.Data,
				Len:  sh.Len,
				Cap:  sh.Len,
			})))
		}
	}
	return m
}

func (mapData *hashMapData[K, V]) indexElement(hashedKey uintptr) *ListElement[K, V] {
	index := hashedKey >> mapData.keyshifts
	ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(mapData.data) + index*intSizeBytes))
	item := (*ListElement[K, V])(atomic.LoadPointer(ptr))
	for (item == nil || hashedKey < item.keyHash) && index > 0 {
		index--
		ptr = (*unsafe.Pointer)(unsafe.Pointer(uintptr(mapData.data) + index*intSizeBytes))
		item = (*ListElement[K, V])(atomic.LoadPointer(ptr))
	}
	return item
}

// Del deletes the key from the map.
func (m *HashMap[K, V]) Del(key K) {
	list := m.linkedlist
	if list == nil {
		return
	}

	h := m.hasher(key)

	element := m.datamap.Load().indexElement(h)
ElementLoop:
	for ; element != nil; element = element.Next() {
		if element.keyHash == h && element.key == key {
			break ElementLoop
		}

		if element.keyHash > h {
			return
		}
	}
	if element == nil {
		return
	}

	m.deleteElement(element)
	list.Delete(element)
}

// deleteElement deletes an element from index
func (m *HashMap[K, V]) deleteElement(element *ListElement[K, V]) {
	for {
		data := m.datamap.Load()
		index := element.keyHash >> data.keyshifts
		ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(data.data) + index*intSizeBytes))

		next := element.Next()
		if next != nil && element.keyHash>>data.keyshifts != index {
			next = nil // do not set index to next item if it's not the same slice index
		}
		atomic.CompareAndSwapPointer(ptr, unsafe.Pointer(element), unsafe.Pointer(next))

		currentdata := m.datamap.Load()
		if data == currentdata { // check that no resize happened
			return
		}
	}
}

// Get retrieves an element from the map under given hash key.
// Using interface{} adds a performance penalty.
// Please consider using GetUintKey or GetStringKey instead.
func (m *HashMap[K, V]) Get(key K) (value V, ok bool) {
	h := m.hasher(key)
	element := m.datamap.Load().indexElement(h)

	// inline HashMap.searchItem()
	for ; element != nil; element = element.Next() {
		if element.keyHash == h && element.key == key {
			value, ok = element.Value(), true
			return
		} else if element.keyHash > h {
			ok = false
			return
		}
	}
	ok = false
	return
}

// Set sets the value under the specified key to the map. An existing item for this key will be overwritten.
// If a resizing operation is happening concurrently while calling Set, the item might show up in the map only after the resize operation is finished.
func (m *HashMap[K, V]) Set(key K, value V) {
	h := m.hasher(key)
	element := &ListElement[K, V]{
		key:     key,
		keyHash: h,
	}
	element.value.Store(&value)
	m.insertListElement(element)
}

func (m *HashMap[K, V]) insertListElement(element *ListElement[K, V]) bool {
	for {
		data := m.datamap.Load()
		existing := data.indexElement(element.keyHash)
		if data == nil {
			m.Grow(DefaultSize)
			continue // read mapdata and slice item again
		}
		list := m.linkedlist

		if !list.AddOrUpdate(element, existing) {
			continue // a concurrent add did interfere, try again
		}

		count := data.addItemToIndex(element)
		if m.resizeNeeded(data, count) && len(m.growChan) == 0 {
			m.growChan <- 0
		}
		return true
	}
}

// adds an item to the index if needed and returns the new item counter if it changed, otherwise 0
func (mapData *hashMapData[K, V]) addItemToIndex(item *ListElement[K, V]) uintptr {
	index := item.keyHash >> mapData.keyshifts
	ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(mapData.data) + index*intSizeBytes))

	for { // loop until the smallest key hash is in the index
		element := (*ListElement[K, V])(atomic.LoadPointer(ptr)) // get the current item in the index
		if element == nil {                                      // no item yet at this index
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
	list := m.linkedlist
	if list == nil {
		return
	}
	first := list.First()
	item := first
	lastIndex := uintptr(0)

	for item != nil {
		index := item.keyHash >> mapData.keyshifts
		if item == first || index != lastIndex { // store item with smallest hash key for every index
			mapData.addItemToIndex(item)
			lastIndex = index
		}
		item = item.Next()
	}
}

// ForEach iterates over key-value pairs and executes the lambda provided for each such pair.
func (m *HashMap[K, V]) ForEach(lambda func(K, V)) {
	list := m.linkedlist
	if list == nil {
		return
	}
	for item := list.First(); item != nil; item = item.Next() {
		lambda(item.key, item.Value())
	}
}

// Grow resizes the hashmap to a new size, gets rounded up to next power of 2.
// To double the size of the hashmap use newSize 0.
// This function returns immediately, the resize operation is done in a goroutine.
// No resizing is done in case of another resize operation already being in progress.
func (m *HashMap[K, V]) Grow(newSize uintptr) {
	if len(m.growChan) == 0 {
		m.growChan <- newSize
	}
}

// SetHasher sets the hash function to the one provided by the user
func (m *HashMap[K, V]) SetHasher(hs func(K) uintptr) {
	m.hasher = hs
}

// Len returns the number of key-value pairs within the map.
func (m *HashMap[K, V]) Len() uintptr {
	l := m.linkedlist
	if l != nil {
		return l.Len()
	} else {
		return 0
	}
}

// Fillrate returns the fill rate of the map as an percentage integer.
func (m *HashMap[K, V]) Fillrate() uintptr {
	data := m.datamap.Load()
	return (data.count.Load() * 100) / data.length
}

func (m *HashMap[K, V]) allocate(newSize uintptr) {
start:
	data := m.datamap.Load()
	if newSize == 0 {
		newSize = data.length << 1
	} else {
		newSize = roundUpPower2(newSize)
	}

	newdata := &hashMapData[K, V]{
		keyshifts: strconv.IntSize - log2(newSize),
		data:      unsafe.Pointer(&make([]*ListElement[K, V], newSize)[0]), // use address of slice data storage
		length:    newSize,
	}

	m.fillIndexItems(newdata) // initialize new index slice with longer keys

	m.datamap.Store(newdata)

	m.fillIndexItems(newdata) // make sure that the new index is up to date with the current state of the linked list

	// check if a new resize needs to be done already
	if m.resizeNeeded(newdata, m.Len()) {
		newSize = 0 // 0 means double the current size
		goto start
	}
}

// a single goroutine per haxmap handling resize operations
func (m *HashMap[K, V]) growRoutine() {
	for newSize := range m.growChan {
		m.allocate(newSize)
	}
}

func (m *HashMap[K, V]) resizeNeeded(data *hashMapData[K, V], count uintptr) bool {
	l := data.length
	if l == 0 {
		return false
	}
	return (count*100)/l > MaxFillRate
}
