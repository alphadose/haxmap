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
	m := New[int, int](0)
	if m.Len() != 0 {
		t.Errorf("new map should be empty but has %d items.", m.Len())
	}

	t.Run("default size is used when zero is provided", func(t *testing.T) {
		m := New[int, int](0)
		index := m.metadata.Load().index
		if len(index) != defaultSize {
			t.Error("map index size is not as expected")
		}
	})
}

func TestOverwrite(t *testing.T) {
	type customUint uint
	m := New[customUint, string](0)
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

func TestSetUint8(t *testing.T) {
	m := New[uint8, string](0)

	for i := 0; i < 10; i++ {
		m.Set(uint8(i), strconv.Itoa(i))
	}

	for i := 1; i <= 10; i++ {
		m.Del(uint8(i))
	}

	for i := 0; i < 10; i++ {
		m.Set(uint8(i), strconv.Itoa(i))
	}

	for i := 0; i < 10; i++ {
		id, ok := m.Get(uint8(i))
		if !ok {
			t.Error("ok should be true for item stored within the map.")
		}
		if id != strconv.Itoa(i) {
			t.Error("item is not as expected.")
		}
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

// From bug https://github.com/alphadose/haxmap/issues/33
func TestSet2(t *testing.T) {
	h := New[int, string](0)
	for i := 1; i <= 10; i++ {
		h.Set(i, strconv.Itoa(i))
	}
	for i := 1; i <= 10; i++ {
		h.Del(i)
	}
	for i := 1; i <= 10; i++ {
		h.Set(i, strconv.Itoa(i))
	}
	for i := 1; i <= 10; i++ {
		id, ok := h.Get(i)
		if !ok {
			t.Error("ok should be true for item stored within the map.")
		}
		if id != strconv.Itoa(i) {
			t.Error("item is not as expected.")
		}
	}
}

func TestGet(t *testing.T) {
	m := New[string, string](0)
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
	m := New[uint, uint](0)
	m.Grow(63)
	d := m.metadata.Load()
	log := int(math.Log2(64))
	expectedSize := uintptr(strconv.IntSize - log)
	if d.keyshifts != expectedSize {
		t.Errorf("Grow operation did not result in correct internal map data structure, Dump -> %#v", d)
	}
}

func TestGrow2(t *testing.T) {
	size := 64
	m := New[int, any](uintptr(size))
	for i := 0; i < 10000; i++ {
		m.Set(i, nil)
		m.Del(i)
		if n := len(m.metadata.Load().index); n != size {
			t.Fatalf("map should not be resized, new size: %d", n)
		}
	}
}

func TestFillrate(t *testing.T) {
	m := New[int, any](0)
	for i := 0; i < 1000; i++ {
		m.Set(i, nil)
	}
	for i := 0; i < 1000; i++ {
		m.Del(i)
	}
	if fr := m.Fillrate(); fr != 0 {
		t.Errorf("Fillrate should be zero when the map is empty, fillrate: %v", fr)
	}
}

func TestDelete(t *testing.T) {
	m := New[int, *Animal](0)
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
	m := New[int, string](0)
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
		m    = New[int, string](0)
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

func TestForEach(t *testing.T) {
	m := New[int, *Animal](0)

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

func TestClear(t *testing.T) {
	m := New[int, any](0)
	for i := 0; i < 100; i++ {
		m.Set(i, nil)
	}
	m.Clear()
	if m.Len() != 0 {
		t.Error("map size should be zero after clear")
	}
	if m.Fillrate() != 0 {
		t.Error("fillrate should be zero after clear")
	}
	log := int(math.Log2(defaultSize))
	expectedSize := uintptr(strconv.IntSize - log)
	if m.metadata.Load().keyshifts != expectedSize {
		t.Error("keyshift is not as expected after clear")
	}
	for i := 0; i < 100; i++ {
		if _, ok := m.Get(i); ok {
			t.Error("the entries should not be existing in the map after clear")
		}
	}
}

func TestMapParallel(t *testing.T) {
	max := 10
	dur := 2 * time.Second
	m := New[int, int](0)
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
	blocks := New[string, struct{}](0)

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
	m := New[string, int](0)
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
	m := New[string, custom](0)
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
	m := New[string, int](0)
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

func TestUint8(t *testing.T) {
	m := New[uint8, string](0)

	m.Set(0, "cat")

	val, ok := m.Get(0)
	if !ok {
		t.Error("Key doesnt exists")
	}
	if val != "cat" {
		t.Error("New value not set")
	}
}

func TestUint64(t *testing.T) {
	m := New[uint64, string](0)

	m.Set(0, "cat")

	val, ok := m.Get(0)
	if !ok {
		t.Error("Key doesnt exists")
	}
	if val != "cat" {
		t.Error("New value not set")
	}
}

func TestUint32(t *testing.T) {
	m := New[uint32, string](0)

	m.Set(0, "cat")

	val, ok := m.Get(0)
	if !ok {
		t.Error("Key doesnt exists")
	}
	if val != "cat" {
		t.Error("New value not set")
	}
}

func TestUintptr(t *testing.T) {
	m := New[uintptr, string](0)

	m.Set(0, "cat")

	val, ok := m.Get(0)
	if !ok {
		t.Error("Key doesnt exists")
	}
	if val != "cat" {
		t.Error("New value not set")
	}

}

func TestString(t *testing.T) {
	m := New[string, string](0)

	m.Set("1", "cat")

	val, ok := m.Get("1")
	if !ok {
		t.Error("Key doesnt exists")
	}
	if val != "cat" {
		t.Error("New value not set")
	}

}

func TestHashStability(t *testing.T) {
	m := New[string, string](0)
	key := "stability_test"
	expectedValue := "value"
	m.Set(key, expectedValue)

	val, ok := m.Get(key)
	if !ok {
		t.Errorf("Expected key %s to exist in the map", key)
	}
	if val != expectedValue {
		t.Errorf("Expected value %s for key %s, got %s", expectedValue, key, val)
	}
}

func TestHashCollision(t *testing.T) {
	m := New[string, string](0)

	key1 := "collision_key_1"
	key2 := "collision_key_2"

	m.Set(key1, "value1")
	m.Set(key2, "value2")

	val1, ok1 := m.Get(key1)
	if !ok1 || val1 != "value1" {
		t.Errorf("Expected value for %s to be 'value1', got %v", key1, val1)
	}

	val2, ok2 := m.Get(key2)
	if !ok2 || val2 != "value2" {
		t.Errorf("Expected value for %s to be 'value2', got %v", key2, val2)
	}
}

func TestHashUinptrCollision(t *testing.T) {
	m := New[uintptr, int](0)
	staticHasher := func(key uintptr) uintptr {
		return 0
	}
	m.SetHasher(staticHasher)
	m.Set(1, 1)
	m.Set(2, 2)
	_, ok := m.Get(1)
	if !ok {
		t.Error("1 not found")
	}
	_, ok = m.Get(2)
	if !ok {
		t.Error("2 not found")
	}
}

func TestMapLargeLoad(t *testing.T) {
	m := New[uintptr, int](0)
	for i := 0; i < 1000000; i++ {
		m.Set(uintptr(i), i)
	}
	if value, ok := m.Get(999999); !ok || value != 999999 {
		t.Errorf("Expected 999999, got %v", value)
	}
}
