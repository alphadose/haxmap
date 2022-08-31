module github.com/alphadose/haxmap/benchmarks

go 1.19

replace github.com/alphadose/haxmap => ../

require (
	github.com/alphadose/haxmap v0.0.0-00010101000000-000000000000
	github.com/cornelk/hashmap v1.0.6
)

require github.com/cespare/xxhash v1.1.0 // indirect
