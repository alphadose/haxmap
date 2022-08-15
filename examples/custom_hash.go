package main

import (
	"github.com/alphadose/haxmap"
)

// your custom hash function
func customStringHasher(s string) uintptr {
	return uintptr(len(s))
}

func main() {
	// initialize a string-string map with your custom hash function
	// this overrides the default xxHash algorithm
	m := haxmap.New[string, string]()
	m.SetHasher(customStringHasher)

	m.Set("one", "1")
	val, ok := m.Get("one")
	if ok {
		println(val)
	}
}
