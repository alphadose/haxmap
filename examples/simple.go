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

	// get the value and print it
	val, ok := mep.Get(1)
	if ok {
		println(val)
	}

	mep.Set(2, "two")
	mep.Set(3, "three")
	mep.Set(4, "four")

	// ForEach loop to iterate over all key-value pairs and execute the given lambda
	mep.ForEach(func(key int, value string) bool {
		fmt.Printf("Key -> %d | Value -> %s\n", key, value)
		return true // return `true` to continue iteration and `false` to break iteration
	})

	mep.Del(1) // delete a value
	mep.Del(0) // delete is safe even if a key doesn't exists

	// bulk deletion is supported too in the same API call
	// has better performance than deleting keys one by one
	mep.Del(2, 3, 4)

	if mep.Len() == 0 {
		println("cleanup complete")
	}
}
