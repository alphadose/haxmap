package main

import (
	"testing"

	"github.com/alphadose/haxmap"
)

func TestHash0Collision(t *testing.T) {
	m := haxmap.New[string, int]()
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
