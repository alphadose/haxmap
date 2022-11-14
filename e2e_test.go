package haxmap

import (
	"fmt"
	"math"
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
	type customUint uint
	m := New[customUint, string]()
	key := customUint(1)
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
	d := m.metadata.Load()
	log := int(math.Log2(64))
	expectedSize := uintptr(strconv.IntSize - log)
	if d.keyshifts != expectedSize {
		t.Errorf("Grow operation did not result in correct internal map data structure, Dump -> %#v", d)
	}
}

func TestDelete(t *testing.T) {
	m := New[int, *Animal]()
	cat := &Animal{"cat"}
	tiger := &Animal{"tiger"}

	m.Set(1, cat)
	m.Set(2, tiger)
	m.Del(0)
	m.Del(3, 4, 5)
	if m.Len() != 2 {
		t.Error("map should contain exactly two elements.")
	}
	m.Del(1, 2, 1)

	if m.Len() != 0 {
		t.Error("map should be empty.")
	}

	_, ok := m.Get(1) // Get a missing element.
	if ok {
		t.Error("ok should be false when item is missing from map.")
	}
}

// From bug https://github.com/alphadose/haxmap/issues/11
func TestDelete2(t *testing.T) {
	m := New[int, string]()
	m.Set(1, "one")
	m.Del(1) // delegate key 1
	if m.Len() != 0 {
		t.Fail()
	}
	// Still can traverse the key/value pair ？
	m.ForEach(func(key int, value string) bool {
		t.Fail()
		return true
	})
}

// from https://pkg.go.dev/sync#Map.LoadOrStore
func TestGetOrSet(t *testing.T) {
	var (
		m    = New[int, string]()
		data = "one"
	)
	if val, loaded := m.GetOrSet(1, data); loaded {
		t.Error("Value should not have been present")
	} else if val != data {
		t.Error("Returned value should be the same as given value if absent")
	}
	if val, loaded := m.GetOrSet(1, data); !loaded {
		t.Error("Value should have been present")
	} else if val != data {
		t.Error("Returned value should be the same as given value")
	}
}

func TestIterator(t *testing.T) {
	m := New[int, *Animal]()

	m.ForEach(func(i int, a *Animal) bool {
		t.Errorf("map should be empty but got key -> %d and value -> %#v.", i, a)
		return true
	})

	itemCount := 16
	for i := itemCount; i > 0; i-- {
		m.Set(i, &Animal{strconv.Itoa(i)})
	}

	counter := 0
	m.ForEach(func(i int, a *Animal) bool {
		if a == nil {
			t.Error("Expecting an object.")
		}
		counter++
		return true
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
		go func(blocks *Map[string, struct{}], i int) {
			defer wg.Done()

			blocks.Set(strconv.Itoa(i), struct{}{})

			wg.Add(1)
			go func(blocks *Map[string, struct{}], i int) {
				defer wg.Done()

				blocks.Get(strconv.Itoa(i))
			}(blocks, i)
		}(blocks, i)
	}

	wg.Wait()
}

// Collision test case when hash key is 0 in value for all entries
func TestHash0Collision(t *testing.T) {
	m := New[string, int]()
	staticHasher := func(key string) uintptr {
		return 0
	}
	m.SetHasher(staticHasher)
	m.Set("1", 1)
	m.Set("2", 2)
	_, ok := m.Get("1")
	if !ok {
		t.Error("1 not found")
	}
	_, ok = m.Get("2")
	if !ok {
		t.Error("2 not found")
	}
}

// test map freezing issue
// https://github.com/alphadose/haxmap/issues/7
// https://github.com/alphadose/haxmap/issues/8
// Update:- Solved now
func TestInfiniteLoop(t *testing.T) {
	t.Run("infinite loop", func(b *testing.T) {
		m := New[int, int](512)
		for i := 0; i < 112050; i++ {
			if i > 112024 {
				m.Set(i, i) // set debug point here and step into until .inject
			} else {
				m.Set(i, i)
			}
		}
	})
}

// https://github.com/alphadose/haxmap/issues/18
// test compare and swap
func TestCAS(t *testing.T) {
	type custom struct {
		val int
	}
	m := New[string, custom]()
	m.Set("1", custom{val: 1})
	if m.CompareAndSwap("1", custom{val: 420}, custom{val: 2}) {
		t.Error("Invalid Compare and Swap")
	}
	if !m.CompareAndSwap("1", custom{val: 1}, custom{val: 2}) {
		t.Error("Compare and Swap Failed")
	}
	val, ok := m.Get("1")
	if !ok {
		t.Error("Key doesnt exists")
	}
	if val.val != 2 {
		t.Error("Invalid Compare and Swap value returned")
	}
}

// https://github.com/alphadose/haxmap/issues/18
// test swap
func TestSwap(t *testing.T) {
	m := New[string, int]()
	m.Set("1", 1)
	val, swapped := m.Swap("1", 2)
	if !swapped {
		t.Error("Swap failed")
	}
	if val != 1 {
		t.Error("Old value not returned in swal")
	}
	val, ok := m.Get("1")
	if !ok {
		t.Error("Key doesnt exists")
	}
	if val != 2 {
		t.Error("New value not set")
	}
}
