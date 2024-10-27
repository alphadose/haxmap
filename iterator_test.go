//go:build go1.23
// +build go1.23

package haxmap

import (
	"testing"
)

func TestIterators(t *testing.T) {
	type Value = struct {
		key int
	}

	m := New[int, *Value]()

	itemCount := 16
	for i := itemCount; i > 0; i-- {
		m.Set(i, &Value{i})
	}

	t.Run("iterator", func(t *testing.T) {
		counter := 0
		for k, v := range m.Iterator() {
			if v == nil {
				t.Error("Expecting an object.")
			} else if k != v.key {
				t.Error("Incorrect key/value pairs")
			}

			counter++
		}

		if counter != itemCount {
			t.Error("Iterated item count did not match.")
		}
	})

	t.Run("keys", func(t *testing.T) {
		counter := 0
		for k := range m.Keys() {
			_, ok := m.Get(k)
			if !ok {
				t.Error("The key is not is the map")
			}
			counter++
		}

		if counter != itemCount {
			t.Error("Iterated item count did not match.")
		}
	})
}
