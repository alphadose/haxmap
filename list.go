package haxmap

import (
	"sync/atomic"
)

// mark a node for being deleted, also used as list_head
// the search() function skips nodes with `keyHash = marked`
const marked = ^uintptr(0)

// Below implementation is a lock-free linked list based on https://www.cl.cam.ac.uk/research/srg/netos/papers/2001-caslists.pdf by Timothy L. Harris
// Performance improvements suggested in https://arxiv.org/pdf/2010.15755.pdf were also added

// newListHead returns the new head of any list
func newListHead[K hashable, V any]() *element[K, V] {
	ptr := atomic.Pointer[element[K, V]]{}
	ptr.Store(nil)
	val := atomic.Pointer[V]{}
	val.Store(new(V))
	return &element[K, V]{marked, *new(K), ptr, val}
}

// a single node in the list
type element[K hashable, V any] struct {
	keyHash uintptr
	key     K
	// The next element in the list. If this pointer has the marked flag set it means THIS element, not the next one, is deleted.
	nextPtr atomic.Pointer[element[K, V]]
	value   atomic.Pointer[V]
}

// next returns the next element
// this also deletes all marked elements while traversing the list
func (self *element[K, V]) next() *element[K, V] {
	for nextElement := self.nextPtr.Load(); nextElement != nil; {
		// if our next element contains marked that means WE are deleted, and we can just return the next-next element
		if nextElement.keyHash == marked {
			return nextElement.next()
		}
		// if our next element is itself deleted (by the same criteria) then we will just replace
		// it with its next() (which should be the first node behind it that isn't itself deleted) and then check again
		if nextElement.isDeleted() {
			self.nextPtr.CompareAndSwap(nextElement, nextElement.next())
			nextElement = self.nextPtr.Load()
		} else {
			return nextElement
		}
	}
	return nil
}

// addBefore inserts an element before the specified element
func (self *element[K, V]) addBefore(t uintptr, allocatedElement, before *element[K, V]) bool {
	if self.next() != before {
		return false
	}
	allocatedElement.nextPtr.Store(before)
	return self.nextPtr.CompareAndSwap(before, allocatedElement)
}

// inject updates an existing value in the list if present or adds a new entry
func (self *element[K, V]) inject(c uintptr, key K, value *V) *element[K, V] {
	var alloc, left, curr, right *element[K, V]
	for {
		left, curr, right = self.search(c, key)
		if curr != nil {
			curr.value.Store(value)
			return curr
		}
		if left != nil {
			if alloc == nil {
				alloc = &element[K, V]{keyHash: c, key: key}
				alloc.value.Store(value)
			}
			if left.addBefore(c, alloc, right) {
				return alloc
			}
		}
	}
}

// search for an element in the list and return left_element, searched_element and right_element respectively
func (self *element[K, V]) search(c uintptr, key K) (*element[K, V], *element[K, V], *element[K, V]) {
	var (
		left, right *element[K, V]
		curr        = self
	)
	for {
		if curr == nil {
			return left, curr, right
		}
		right = curr.next()
		if curr.keyHash != marked {
			if c < curr.keyHash {
				right = curr
				curr = nil
				return left, curr, right
			} else if c == curr.keyHash && key == curr.key {
				return left, curr, right
			}
		}
		left = curr
		curr = left.next()
		right = nil
	}
}

// fastSearch is a fast version of the search returning only the desired element
// this search mechanism also skips over deleted elements and does not try to remove them
func (self *element[K, V]) fastSearch(c uintptr, key K) *element[K, V] {
	var (
		left *element[K, V]
		curr = self
	)
	for curr != nil {
		if curr.keyHash < c || curr.keyHash == marked {
			left = curr
			curr = left.next()
			continue
		}
		if curr.keyHash == c && curr.key == key {
			return curr
		}
		if curr.keyHash > c {
			return nil
		}
	}
	return nil
}

// remove removes the current node
func (self *element[K, V]) remove() bool {
	for {
		if self.next() == nil {
			return false
		}
		if self.add(marked) {
			self.next()
			return true
		}
	}
}

// if current element is deleted
func (self *element[K, V]) isDeleted() bool {
	next := self.nextPtr.Load()
	if next == nil {
		return false
	}
	if next.keyHash == marked {
		return true
	}
	return false
}

func (self *element[K, V]) add(c uintptr) bool {
	alloc := &element[K, V]{keyHash: c}
	for {
		// If we are deleted then we do not allow adding new children.
		if self.isDeleted() {
			return false
		}
		// If we succeed in adding before our perceived next, just return true.
		if self.addBefore(c, alloc, self.next()) {
			return true
		}
	}
}
