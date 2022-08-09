package haxmap

import (
	"reflect"
	"strconv"
	"sync/atomic"
	"unsafe"

	"github.com/alphadose/haxmap/hash"
)

const (
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
		int | uint | uintptr | string
	}

	hashMapData[K hashable, V any] struct {
		keyshifts uintptr              // Pointer size - log2 of array size, to be used as index in the data array
		count     atomic.Uintptr       // count of filled elements in the slice
		data      unsafe.Pointer       // pointer to slice data array
		index     []*ListElement[K, V] // storage for the slice for the garbage collector to not clean it up
	}

	// HashMap implements a read optimized hash map.
	HashMap[K hashable, V any] struct {
		Hasher     func(K) uintptr
		datamap    atomic.Pointer[hashMapData[K, V]] // pointer to a map instance that gets replaced if the map resizes
		linkedlist atomic.Pointer[List[K, V]]        // key sorted linked list of elements
		resizing   atomic.Uintptr                    // flag that marks a resizing operation in progress
	}
)

// New returns a new HashMap instance with an optional specific initialization size.
func New[K hashable, V any](size ...uintptr) *HashMap[K, V] {
	m := &HashMap[K, V]{}
	if len(size) > 0 {
		m.allocate(size[0])
	}
	switch reflect.TypeOf(*new(K)).Name() {
	case "string":
		m.Hasher = func(key K) uintptr {
			sh := (*reflect.StringHeader)(unsafe.Pointer(&key))
			return hash.Sum(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: sh.Data,
				Len:  sh.Len,
				Cap:  sh.Len,
			})))
		}
	default:
		m.Hasher = func(key K) uintptr {
			return hash.Sum(*(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(unsafe.Pointer(&key)),
				Len:  intSizeBytes,
				Cap:  intSizeBytes,
			})))
		}
	}
	return m
}

// Len returns the number of key-value pairs within the map.
func (m *HashMap[K, V]) Len() uintptr {
	l := m.list()
	if l != nil {
		return l.Len()
	} else {
		return 0
	}
}

func (m *HashMap[K, V]) mapData() *hashMapData[K, V] {
	return m.datamap.Load()
}

func (m *HashMap[K, V]) list() *List[K, V] {
	return m.linkedlist.Load()
}

func (m *HashMap[K, V]) allocate(newSize uintptr) {
	list := NewList[K, V]()
	// atomic swap in case of another allocation happening concurrently
	if m.linkedlist.CompareAndSwap(nil, list) {
		if m.resizing.CompareAndSwap(0, 1) {
			m.grow(newSize, false)
		}
	}
}

// Fillrate returns the fill rate of the map as an percentage integer.
func (m *HashMap[K, V]) Fillrate() uintptr {
	data := m.mapData()
	count := data.count.Load()
	l := uintptr(len(data.index))
	return (count * 100) / l
}

func (m *HashMap[K, V]) resizeNeeded(data *hashMapData[K, V], count uintptr) bool {
	l := uintptr(len(data.index))
	if l == 0 {
		return false
	}
	fillRate := (count * 100) / l
	return fillRate > MaxFillRate
}

func (m *HashMap[K, V]) indexElement(hashedKey uintptr) (data *hashMapData[K, V], item *ListElement[K, V]) {
	data = m.mapData()
	if data == nil {
		return nil, nil
	}
	index := hashedKey >> data.keyshifts
	ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(data.data) + index*intSizeBytes))
	item = (*ListElement[K, V])(atomic.LoadPointer(ptr))
	for (item == nil || hashedKey < item.keyHash) && index > 0 {
		index--
		ptr = (*unsafe.Pointer)(unsafe.Pointer(uintptr(data.data) + index*intSizeBytes))
		item = (*ListElement[K, V])(atomic.LoadPointer(ptr))
	}
	return data, item
}

// Del deletes the key from the map.
func (m *HashMap[K, V]) Del(key K) {
	list := m.list()
	if list == nil {
		return
	}

	h := m.Hasher(key)

	var element *ListElement[K, V]
ElementLoop:
	for _, element = m.indexElement(h); element != nil; element = element.Next() {
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
		data := m.mapData()
		index := element.keyHash >> data.keyshifts
		ptr := (*unsafe.Pointer)(unsafe.Pointer(uintptr(data.data) + index*intSizeBytes))

		next := element.Next()
		if next != nil && element.keyHash>>data.keyshifts != index {
			next = nil // do not set index to next item if it's not the same slice index
		}
		atomic.CompareAndSwapPointer(ptr, unsafe.Pointer(element), unsafe.Pointer(next))

		currentdata := m.mapData()
		if data == currentdata { // check that no resize happened
			break
		}
	}
}

// Get retrieves an element from the map under given hash key.
// Using interface{} adds a performance penalty.
// Please consider using GetUintKey or GetStringKey instead.
func (m *HashMap[K, V]) Get(key K) (value V, ok bool) {
	h := m.Hasher(key)
	data, element := m.indexElement(h)
	if data == nil {
		return *new(V), false
	}

	// inline HashMap.searchItem()
	for element != nil {
		if element.keyHash == h && element.key == key {
			return element.Value(), true
		}

		if element.keyHash > h {
			return *new(V), false
		}

		element = element.Next()
	}
	return *new(V), false
}

// Set sets the value under the specified key to the map. An existing item for this key will be overwritten.
// If a resizing operation is happening concurrently while calling Set, the item might show up in the map only after the resize operation is finished.
func (m *HashMap[K, V]) Set(key K, value V) {
	h := m.Hasher(key)
	element := &ListElement[K, V]{
		key:     key,
		keyHash: h,
	}
	element.value.Store(&value)
	m.insertListElement(element)
}

func (m *HashMap[K, V]) insertListElement(element *ListElement[K, V]) bool {
	for {
		data, existing := m.indexElement(element.keyHash)
		if data == nil {
			m.allocate(DefaultSize)
			continue // read mapdata and slice item again
		}
		list := m.list()

		if !list.AddOrUpdate(element, existing) {
			continue // a concurrent add did interfere, try again
		}

		count := data.addItemToIndex(element)
		if m.resizeNeeded(data, count) {
			if m.resizing.CompareAndSwap(0, 1) {
				go m.grow(0, true)
			}
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

// Grow resizes the hashmap to a new size, gets rounded up to next power of 2.
// To double the size of the hashmap use newSize 0.
// This function returns immediately, the resize operation is done in a goroutine.
// No resizing is done in case of another resize operation already being in progress.
func (m *HashMap[K, V]) Grow(newSize uintptr) {
	if m.resizing.CompareAndSwap(0, 1) {
		go m.grow(newSize, true)
	}
}

func (m *HashMap[K, V]) grow(newSize uintptr, loop bool) {
	for {
		data := m.mapData()
		if newSize == 0 {
			newSize = uintptr(len(data.index)) << 1
		} else {
			newSize = roundUpPower2(newSize)
		}

		index := make([]*ListElement[K, V], newSize)
		header := (*reflect.SliceHeader)(unsafe.Pointer(&index))

		newdata := &hashMapData[K, V]{
			keyshifts: strconv.IntSize - log2(newSize),
			data:      unsafe.Pointer(header.Data), // use address of slice data storage
			index:     index,
		}

		m.fillIndexItems(newdata) // initialize new index slice with longer keys

		m.datamap.Store(newdata)

		m.fillIndexItems(newdata) // make sure that the new index is up to date with the current state of the linked list

		if !loop {
			break
		}

		// check if a new resize needs to be done already
		if !m.resizeNeeded(newdata, m.Len()) {
			break
		}
		newSize = 0 // 0 means double the current size
	}
	m.resizing.CompareAndSwap(1, 0)
}

func (m *HashMap[K, V]) fillIndexItems(mapData *hashMapData[K, V]) {
	list := m.list()
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
	list := m.list()
	if list == nil {
		return
	}
	for item := list.First(); item != nil; item = item.Next() {
		lambda(item.key, item.Value())
	}
}
