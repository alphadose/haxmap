package haxmap

import (
	"sync/atomic"
)

const maxUintptr = ^uintptr(0)

// List is a sorted doubly linked list.
type List[K hashable, V any] struct {
	count atomic.Uintptr
	head  *ListElement[K, V]
}

// NewList returns an initialized list.
func NewList[K hashable, V any]() *List[K, V] {
	return &List[K, V]{head: &ListElement[K, V]{}}
}

// ListElement is an element of a list.
type ListElement[K hashable, V any] struct {
	deleted         uint32 // marks the item as deleting or deleted
	keyHash         uintptr
	key             K
	previousElement atomic.Pointer[ListElement[K, V]] // is nil for the first item in list
	nextElement     atomic.Pointer[ListElement[K, V]] // is nil for the last item in list
	value           atomic.Pointer[V]
}

// Value returns the value of the list item.
func (e *ListElement[K, V]) Value() (value V) {
	value = *e.value.Load()
	return
}

// Next returns the item on the right.
func (e *ListElement[K, V]) Next() *ListElement[K, V] {
	return e.nextElement.Load()
}

// Previous returns the item on the left.
func (e *ListElement[K, V]) Previous() *ListElement[K, V] {
	return e.previousElement.Load()
}

// setValue sets the value of the item.
// The value needs to be wrapped in unsafe.Pointer already.
func (e *ListElement[K, V]) setValue(value *V) {
	e.value.Store(value)
}

// Len returns the number of elements within the list.
func (l *List[K, V]) Len() uintptr {
	return l.count.Load()
}

// First returns the first item of the list.
func (l *List[K, V]) First() *ListElement[K, V] {
	return l.head.Next()
}

// AddOrUpdate adds or updates an item to the list.
func (l *List[K, V]) AddOrUpdate(element *ListElement[K, V], searchStart *ListElement[K, V]) bool {
	left, found, right := l.search(searchStart, element)
	if found != nil { // existing item found
		found.setValue(element.value.Load()) // update the value
		return true
	}
	return l.insertAt(element, left, right)
}

func (l *List[K, V]) search(searchStart, item *ListElement[K, V]) (left, found, right *ListElement[K, V]) {
	if searchStart != nil && item.keyHash < searchStart.keyHash { // key would remain left from item? {
		searchStart = nil // start search at head
	}

	if searchStart == nil { // start search at head?
		left = l.head
		found = left.Next()
		if found == nil { // no items beside head?
			return nil, nil, nil
		}
	} else {
		found = searchStart
	}

	for {
		if item.keyHash == found.keyHash { // key already exists
			return nil, found, nil
		}

		if item.keyHash < found.keyHash { // new item needs to be inserted before the found value
			if l.head == left {
				return nil, nil, found
			}
			return left, nil, found
		}

		// go to next element in sorted linked list
		left = found
		found = left.Next()
		if found == nil { // no more items on the right
			return left, nil, nil
		}
	}
}

func (l *List[K, V]) insertAt(element *ListElement[K, V], left *ListElement[K, V], right *ListElement[K, V]) bool {
	if left == nil {
		//element->previous = head
		element.previousElement.Store(l.head)
		//element->next = right
		element.nextElement.Store(right)

		// insert at head, head-->next = element
		if !l.head.nextElement.CompareAndSwap(right, element) {
			return false // item was modified concurrently
		}

		//right->previous = element
		if right != nil {
			if !right.previousElement.CompareAndSwap(l.head, element) {
				return false // item was modified concurrently
			}
		}
	} else {
		element.previousElement.Store(left)
		element.nextElement.Store(right)

		if !left.nextElement.CompareAndSwap(right, element) {
			return false // item was modified concurrently
		}

		if right != nil {
			if !right.previousElement.CompareAndSwap(left, element) {
				return false // item was modified concurrently
			}
		}
	}

	l.count.Add(1)
	return true
}

// Delete deletes an element from the list.
func (l *List[K, V]) Delete(element *ListElement[K, V]) {
	if !atomic.CompareAndSwapUint32(&element.deleted, 0, 1) {
		return // concurrent delete of the item in progress
	}

	for {
		left := element.Previous()
		right := element.Next()

		if left == nil { // element is first item in list?
			if !l.head.nextElement.CompareAndSwap(element, right) {
				continue // now head item was inserted concurrently
			}
		} else if !left.nextElement.CompareAndSwap(element, right) {
			continue // item was modified concurrently
		}
		if right != nil {
			right.previousElement.CompareAndSwap(element, left)
		}
		break
	}

	l.count.Add(maxUintptr) // decrease counter
}
