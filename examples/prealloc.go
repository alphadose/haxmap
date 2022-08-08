package main

import (
	"github.com/alphadose/haxmap"
)

func main() {
	const initialSize = 1 << 10

	// pre-allocating the size of the map will prevent all grow operations
	// until that limit is hit thereby improving performance
	m := haxmap.New[int, string](initialSize)

	m.Set(1, "1")
	val, ok := m.Get(1)
	if ok {
		println(val)
	}
}
