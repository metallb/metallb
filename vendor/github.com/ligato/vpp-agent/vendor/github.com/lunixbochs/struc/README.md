[![Build Status](https://travis-ci.org/lunixbochs/struc.svg?branch=master)](https://travis-ci.org/lunixbochs/struc)

struc
====

Struc exists to pack and unpack C-style structures from bytes, which is useful for binary files and network protocols. It could be considered an alternative to `encoding/binary`, which requires massive boilerplate for some similar operations.

Take a look at an [example comparing `struc` and `encoding/binary`](https://bochs.info/p/cxvm9)

Struc considers usability first. That said, it does cache reflection data and aims to be competitive with `encoding/binary` struct packing in every way, including performance.

Example struct
----

```Go
type Example struct {
    Var   int `struc:"int32,sizeof=Str"`
    Str   string
    Weird []byte `struc:"[8]int64"`
    Var   []int `struc:"[]int32,little"`
}
```

Struct tag format
----

 - ```Var []int `struc:"[]int32,little,sizeof=StringField"` ``` will pack Var as a slice of little-endian int32, and link it as the size of `StringField`.
 - `sizeof=`: Indicates this field is a number used to track the length of a another field. `sizeof` fields are automatically updated on `Pack()` based on the current length of the tracked field, and are used to size the target field during `Unpack()`.
 - Bare values will be parsed as type and endianness.

Endian formats
----

 - `big` (default)
 - `little`

Recognized types
----

 - `pad` - this type ignores field contents and is backed by a `[length]byte` containing nulls
 - `bool`
 - `byte`
 - `int8`, `uint8`
 - `int16`, `uint16`
 - `int32`, `uint32`
 - `int64`, `uint64`
 - `float32`
 - `float64`

Types can be indicated as arrays/slices using `[]` syntax. Example: `[]int64`, `[8]int32`.

Bare slice types (those with no `[size]`) must have a linked `Sizeof` field.

Private fields are ignored when packing and unpacking.

Example code
----

```Go
package main

import (
    "bytes"
    "github.com/lunixbochs/struc"
)

type Example struct {
    A int `struc:"big"`

    // B will be encoded/decoded as a 16-bit int (a "short")
    // but is stored as a native int in the struct
    B int `struc:"int16"`

    // the sizeof key links a buffer's size to any int field
    Size int `struc:"int8,little,sizeof=Str"`
    Str  string

    // you can get freaky if you want
    Str2 string `struc:"[5]int64"`
}

func main() {
    var buf bytes.Buffer
    t := &Example{1, 2, 0, "test", "test2"}
    err := struc.Pack(&buf, t)
    o := &Example{}
    err = struc.Unpack(&buf, o)
}
```

Benchmark
----

`BenchmarkEncode` uses struc. `Stdlib` benchmarks use equivalent `encoding/binary` code. `Manual` encodes without any reflection, and should be considered an upper bound on performance (which generated code based on struc definitions should be able to achieve).

```
BenchmarkEncode        1000000   1265 ns/op
BenchmarkStdlibEncode  1000000   1855 ns/op
BenchmarkManualEncode  5000000    284 ns/op
BenchmarkDecode        1000000   1259 ns/op
BenchmarkStdlibDecode  1000000   1656 ns/op
BenchmarkManualDecode  20000000  89.0 ns/op
```
