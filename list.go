package haxmap

import (
	"sync/atomic"
)

// states denoting whether a node is deleted or not
const (
	notDeleted uint32 = iota
	deleted
)

// Below implementation is a lock-free linked list based on https://www.cl.cam.ac.uk/research/srg/netos/papers/2001-caslists.pdf by Timothy L. Harris
// Performance improvements suggested in https://arxiv.org/pdf/2010.15755.pdf were also added

// newListHead returns the new head of any list
func newListHead[K Hashable, V any]() *element[K, V] {
	e := &element[K, V]{}
	e.nextPtr.Store(nil)
	e.value.Store(new(V))
	return e
}

// a single node in the list
type element[K Hashable, V any] struct {
	key K

	keyHash uintptr

	value atomicPointer[V]

	nextPtr atomicPointer[element[K, V]]

	deleted uint32
}

// next returns the next element
// this also deletes all marked elements while traversing the list
func (self *element[K, V]) next() *element[K, V] {
	for {
		nextElement := self.nextPtr.Load()
		if nextElement == nil || !nextElement.isDeleted() {
			return nextElement
		}

		if self.nextPtr.CompareAndSwap(nextElement, nextElement.nextPtr.Load()) {
			continue
		}
	}
}

// addBefore inserts an element before the specified element
func (self *element[K, V]) addBefore(allocatedElement, before *element[K, V]) bool {
	if self.next() != before {
		return false
	}
	allocatedElement.nextPtr.Store(before)
	return self.nextPtr.CompareAndSwap(before, allocatedElement)
}

// inject updates an existing value in the list if present or adds a new entry
func (self *element[K, V]) inject(c uintptr, key K, value *V) (alloc *element[K, V], _ bool) {
	var (
		left, curr, right = self.search(c, key)
	)
	if curr != nil {
		curr.value.Store(value)
		return curr, false
	}
	if left != nil {
		alloc = &element[K, V]{keyHash: c, key: key}
		alloc.value.Store(value)
		if left.addBefore(alloc, right) {
			return alloc, true
		}
	}
	return nil, false
}

// search for an element in the list and return left_element, searched_element and right_element respectively
func (self *element[K, V]) search(c uintptr, key K) (left *element[K, V], _ *element[K, V], right *element[K, V]) {
	var (
		curr = self
	)
	for {
		if curr == nil {
			return left, curr, right
		}
		right = curr.next()
		if c < curr.keyHash {
			right = curr
			curr = nil
			return left, curr, right
		} else if c == curr.keyHash && key == curr.key {
			return left, curr, right
		}
		left = curr
		curr = left.next()
		right = nil

	}
}

// remove marks a node for deletion
// the node will be removed in the next iteration via `element.next()`
// CAS ensures each node can be marked for deletion exactly once
func (self *element[K, V]) remove() bool {
	return atomic.CompareAndSwapUint32(&self.deleted, notDeleted, deleted)
}

// if current element is deleted
func (self *element[K, V]) isDeleted() bool {
	return atomic.LoadUint32(&self.deleted) == deleted
}
