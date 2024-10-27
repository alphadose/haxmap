//go:build go1.23
// +build go1.23

package haxmap

import "iter"

func (m *Map[K, V]) Iterator() iter.Seq2[K, V] {
	return func(yield func(key K, value V) bool) {
		for item := m.listHead.next(); item != nil; item = item.next() {
			if !yield(item.key, *item.value.Load()) {
				return
			}
		}
	}
}

func (m *Map[K, _]) Keys() iter.Seq[K] {
	return func(yield func(key K) bool) {
		for item := m.listHead.next(); item != nil; item = item.next() {
			if !yield(item.key) {
				return
			}
		}
	}
}
