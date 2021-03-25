# bytespool

[![Go Reference](https://pkg.go.dev/badge/github.com/intuitivelabs/bytespool.svg)](https://pkg.go.dev/github.com/intuitivelabs/bytespool)

The bytespool package provides a simple wrapper over sync.Pool for pools
 containing byte slices of different sizes.

When a byte pool is initialised a minimum and maximum size are provided and
 a round-up factor. Each time a byte slice is requested, if the size is
 between the configured minimum and maximum,  a byte slice of the
 round-up(size, round-up factor) will be returned. If the size is less then
 the minimum size, then the slice will have round-up(minimum size).
For values greater then the maximum size, no sync.Pool will be used, instead
 it will be either "allocated" directly or nil will be returned, depending
  on the request parameters.

## Example

```
	var bPool bytespool.Bpool
	
	bPool.Init(0, 16384, 16) // round-to 16, min 0, max 16384

	b, _ := bPool.Get(5, true) // will return [16]bytes
	bPool.Put(b) // release
	// will return [1024]bytes
	// try 1024 bytes, but only if available
	b , ok = bPool.Get(1024, false) // on success [1024]bytes, true
	if ok {
		bPool.Put(b)
	}
	ok = bPool.Put(make([]byte, 32768) // fails because size is too big
	b, ok = bPool.Get(32768, false) // fails, size too big
	b, ok = bPool.Get(32768, true) // succeeds (forced), but value "allocated"
```
