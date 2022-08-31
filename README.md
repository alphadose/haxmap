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

Benchmarks were performed against [golang sync.Map](https://pkg.go.dev/sync#Map) and the latest [cornelk-hashmap](https://github.com/cornelk/hashmap)

All results were computed from [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) of 20 runs (code available [here](./benchmarks))

1. Concurrent Reads Only
```
name                         time/op
HaxMapReadsOnly-8            8.75µs ± 9%
GoSyncMapReadsOnly-8         22.0µs ±11%
CornelkMapReadsOnly-8        9.20µs ±10%
```


2. Concurrent Reads with Writes
```
name                         time/op
HaxMapReadsWithWrites-8      10.0µs ± 9%
GoSyncMapReadsWithWrites-8   24.8µs ±11%
CornelkMapReadsWithWrites-8  10.5µs ± 9%

name                         alloc/op
HaxMapReadsWithWrites-8      1.29kB ± 6%
GoSyncMapReadsWithWrites-8   6.20kB ± 5%
CornelkMapReadsWithWrites-8  1.59kB ±10%

name                         allocs/op
HaxMapReadsWithWrites-8         161 ± 4%
GoSyncMapReadsWithWrites-8      574 ± 5%
CornelkMapReadsWithWrites-8     198 ±10%
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
