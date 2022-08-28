package haxmap

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type Animal struct {
	name string
}

func TestMapCreation(t *testing.T) {
	m := New[int, int]()
	if m.Len() != 0 {
		t.Errorf("new map should be empty but has %d items.", m.Len())
	}
}

func TestOverwrite(t *testing.T) {
	m := New[uint, string]()
	key := uint(1)
	cat := "cat"
	tiger := "tiger"

	m.Set(key, cat)
	m.Set(key, tiger)

	if m.Len() != 1 {
		t.Errorf("map should contain exactly one element but has %v items.", m.Len())
	}

	item, ok := m.Get(key) // Retrieve inserted element.
	if !ok {
		t.Error("ok should be true for item stored within the map.")
	}
	if item != tiger {
		t.Error("wrong item returned.")
	}
}

func TestSet(t *testing.T) {
	m := New[int, string](4)

	m.Set(4, "cat")
	m.Set(3, "cat")
	m.Set(2, "tiger")
	m.Set(1, "tiger")

	if m.Len() != 4 {
		t.Error("map should contain exactly 4 elements.")
	}
}

func TestGet(t *testing.T) {
	m := New[string, string]()
	cat := "cat"
	key := "animal"

	_, ok := m.Get(key) // Get a missing element.
	if ok {
		t.Error("ok should be false when item is missing from map.")
	}

	m.Set(key, cat)

	_, ok = m.Get("human") // Get a missing element.
	if ok {
		t.Error("ok should be false when item is missing from map.")
	}

	value, ok := m.Get(key) // Retrieve inserted element.
	if !ok {
		t.Error("ok should be true for item stored within the map.")
	}

	if value != cat {
		t.Error("item was modified.")
	}
}

func TestGrow(t *testing.T) {
	m := New[uint, uint]()
	m.Grow(63)

	// make sure to wait for resize operation to finish
	time.Sleep(43 * time.Millisecond)

	d := m.datamap.Load()
	if d.keyshifts != 58 {
		t.Error("Grow operation did not result in correct internal map data structure.")
	}
}

func TestDelete(t *testing.T) {
	m := New[int, *Animal]()

	cat := &Animal{"cat"}
	tiger := &Animal{"tiger"}
	m.Set(1, cat)
	m.Set(2, tiger)
	m.Del(0)
	m.Del(3)
	if m.Len() != 2 {
		t.Error("map should contain exactly two elements.")
	}

	m.Del(1)
	m.Del(1)
	m.Del(2)

	// traverse the map once to remove deleted nodes
	// this is how haxmap lazily removes deleted nodes
	// for k := m.listHead; k != nil; k = k.next() {
	// }

	if m.Len() != 0 {
		t.Error("map should be empty.")
	}

	val, ok := m.Get(1) // Get a missing element.
	if ok {
		t.Error("ok should be false when item is missing from map.")
	}
	if val != nil {
		t.Error("Missing values should return as nil.")
	}
}

func TestIterator(t *testing.T) {
	m := New[int, *Animal]()

	m.ForEach(func(i int, a *Animal) {
		t.Errorf("map should be empty but got key -> %d and value -> %#v.", i, a)
	})

	itemCount := 16
	for i := itemCount; i > 0; i-- {
		m.Set(i, &Animal{strconv.Itoa(i)})
	}

	counter := 0
	m.ForEach(func(i int, a *Animal) {
		if a == nil {
			t.Error("Expecting an object.")
		}
		counter++
	})

	if counter != itemCount {
		t.Error("Returned item count did not match.")
	}
}

func TestMapParallel(t *testing.T) {
	max := 10
	dur := 2 * time.Second
	m := New[int, int]()
	do := func(t *testing.T, max int, d time.Duration, fn func(*testing.T, int)) <-chan error {
		t.Helper()
		done := make(chan error)
		var times int64
		// This goroutines will terminate test in case if closure hangs.
		go func() {
			for {
				select {
				case <-time.After(d + 500*time.Millisecond):
					if atomic.LoadInt64(&times) == 0 {
						done <- fmt.Errorf("closure was not executed even once, something blocks it")
					}
					close(done)
				case <-done:
				}
			}
		}()
		go func() {
			timer := time.NewTimer(d)
			defer timer.Stop()
		InfLoop:
			for {
				for i := 0; i < max; i++ {
					select {
					case <-timer.C:
						break InfLoop
					default:
					}
					fn(t, i)
					atomic.AddInt64(&times, 1)
				}
			}
			close(done)
		}()
		return done
	}
	wait := func(t *testing.T, done <-chan error) {
		t.Helper()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
	// Initial fill.
	for i := 0; i < max; i++ {
		m.Set(i, i)
	}
	t.Run("set_get", func(t *testing.T) {
		doneSet := do(t, max, dur, func(t *testing.T, i int) {
			m.Set(i, i)
		})
		doneGet := do(t, max, dur, func(t *testing.T, i int) {
			if _, ok := m.Get(i); !ok {
				t.Errorf("missing value for key: %d", i)
			}
		})
		wait(t, doneSet)
		wait(t, doneGet)
	})
	t.Run("delete", func(t *testing.T) {
		doneDel := do(t, max, dur, func(t *testing.T, i int) {
			m.Del(i)
		})
		wait(t, doneDel)
	})
}

func TestMapConcurrentWrites(t *testing.T) {
	blocks := New[string, struct{}]()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {

		wg.Add(1)
		go func(blocks *HashMap[string, struct{}], i int) {
			defer wg.Done()

			blocks.Set(strconv.Itoa(i), struct{}{})

			wg.Add(1)
			go func(blocks *HashMap[string, struct{}], i int) {
				defer wg.Done()

				blocks.Get(strconv.Itoa(i))
			}(blocks, i)
		}(blocks, i)
	}

	wg.Wait()
}
