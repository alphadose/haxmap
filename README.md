HaxMap

> A blazing fast concurrent hashmap

This work is derived from [cornelk-hashmap](https://github.com/cornelk/hashmap) and further performance and API improvements have been made

## Installation

You need Golang [1.19.x](https://go.dev/dl/) or above

```bash
$ go get github.com/alphadose/haxmap
```

## Usage

```go
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
```

## Benchmarks

Benchmarks were performed against [golang sync.Map](https://pkg.go.dev/sync#Map) and [cornelk-hashmap](https://github.com/cornelk/hashmap)

All results were computed from [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) of 30 runs (code available [here](./benchmarks))

1. Concurrent Reads Only
```
name                         time/op
HaxMapReadsOnly-8            11.1µs ±12%
GoSyncMapReadsOnly-8         22.0µs ±13%
CornelkMapReadsOnly-8        16.7µs ± 6%

name                         alloc/op
HaxMapReadsOnly-8             0.00B
GoSyncMapReadsOnly-8          0.00B
CornelkMapReadsOnly-8         7.43B ± 8%

name                         allocs/op
HaxMapReadsOnly-8              0.00
GoSyncMapReadsOnly-8           0.00
CornelkMapReadsOnly-8          0.00
```


2. Concurrent Reads with Writes
```
name                         time/op
HaxMapReadsWithWrites-8      13.1µs ±11%
GoSyncMapReadsWithWrites-8   25.0µs ±12%
CornelkMapReadsWithWrites-8  20.0µs ± 6%

name                         alloc/op
HaxMapReadsWithWrites-8      6.71kB ± 9%
GoSyncMapReadsWithWrites-8   6.32kB ± 6%
CornelkMapReadsWithWrites-8  10.0kB ± 9%

name                         allocs/op
HaxMapReadsWithWrites-8         239 ± 9%
GoSyncMapReadsWithWrites-8      585 ± 6%
CornelkMapReadsWithWrites-8     407 ± 9%
```

From the above results it is evident that `haxmap` is currently the fastest golang concurrent hashmap having the least number of `allocs/op` and low dynamic memory footprint

## Tips

1. HaxMap by default uses [xxHash](https://github.com/cespare/xxhash) algorithm, but you can override this and plug-in your own custom hash function. Beneath lies an example for the same.
```go
package main

import (
	"github.com/alphadose/haxmap"
)

// your custom hash function
// the hash function signature must adhere to `func(keyType) uintptr`
func customStringHasher(s string) uintptr {
	return uintptr(len(s))
}

func main() {
	m := haxmap.New[string, string]() // initialize a string-string map
	m.SetHasher(customStringHasher) // this overrides the default xxHash algorithm

	m.Set("one", "1")
	val, ok := m.Get("one")
	if ok {
		println(val)
	}
}
```

2. You can pre-allocate the size of the map which will improve performance in some cases.
```go
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
```
