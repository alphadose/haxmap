# HaxMap

[![Main Actions Status](https://github.com/alphadose/haxmap/workflows/Go/badge.svg)](https://github.com/alphadose/haxmap/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/alphadose/haxmap)](https://goreportcard.com/report/github.com/alphadose/haxmap)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE.md)
> A blazing fast concurrent hashmap

The hashing algorithm used was [xxHash](https://github.com/Cyan4973/xxHash) and the hashmap's buckets were implemented using [Harris lock-free list](https://www.cl.cam.ac.uk/research/srg/netos/papers/2001-caslists.pdf)

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

Benchmarks were performed against [golang sync.Map](https://pkg.go.dev/sync#Map) and the latest generics-enabled [cornelk-hashmap](https://github.com/cornelk/hashmap)

All results were computed from [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) of 20 runs (code available [here](./benchmarks))

1. Concurrent Reads Only
```
name                         time/op
HaxMapReadsOnly-8            9.90µs ±13%
GoSyncMapReadsOnly-8         22.2µs ±14%
CornelkMapReadsOnly-8        10.4µs ±16%

name                         alloc/op
HaxMapReadsOnly-8             0.00B
GoSyncMapReadsOnly-8          0.00B
CornelkMapReadsOnly-8         0.00B

name                         allocs/op
HaxMapReadsOnly-8              0.00
GoSyncMapReadsOnly-8           0.00
CornelkMapReadsOnly-8          0.00
```


2. Concurrent Reads with Writes
```
name                         time/op
HaxMapReadsWithWrites-8      11.2µs ±16%
GoSyncMapReadsWithWrites-8   25.6µs ±14%
CornelkMapReadsWithWrites-8  12.1µs ±12%

name                         alloc/op
HaxMapReadsWithWrites-8      1.35kB ± 8%
GoSyncMapReadsWithWrites-8   6.07kB ± 8%
CornelkMapReadsWithWrites-8  5.80kB ±10%

name                         allocs/op
HaxMapReadsWithWrites-8         168 ± 8%
GoSyncMapReadsWithWrites-8      562 ± 8%
CornelkMapReadsWithWrites-8     207 ±10%
```

From the above results it is evident that `haxmap` takes the least time, memory and allocations in all cases making it the best golang concurrent hashmap in this period of time

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
