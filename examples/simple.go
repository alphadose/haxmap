package main

import (
	"fmt"

	"github.com/alphadose/haxmap"
)

func main() {
	// initialize map with key type `int` and value type `string`
	mep := haxmap.New[int, string]()

	// set a value (overwrites existing value if present)
	mep.Set(1, "one")
	println(mep.Len())
	// get the value and print it
	val, ok := mep.Get(1)
	if ok {
		println(val)
	}

	mep.Set(2, "two")
	mep.Set(3, "three")

	// ForEach loop to iterate over all key-value pairs and execute the given lambda
	mep.ForEach(func(key int, value string) {
		fmt.Printf("Key -> %d | Value -> %s\n", key, value)
	})

	// delete values
	mep.Del(1)
	mep.Del(2)
	mep.Del(3)
	mep.Del(0) // delete is safe even if a key doesn't exists

	if mep.Len() == 0 {
		println("cleanup complete")
	}
}
