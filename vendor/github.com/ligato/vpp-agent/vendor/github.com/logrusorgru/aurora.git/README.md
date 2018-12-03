Aurora
======

[![GoDoc](https://godoc.org/github.com/logrusorgru/aurora?status.svg)](https://godoc.org/github.com/logrusorgru/aurora)
[![WTFPL License](https://img.shields.io/badge/license-wtfpl-blue.svg)](http://www.wtfpl.net/about/)
[![Build Status](https://travis-ci.org/logrusorgru/aurora.svg)](https://travis-ci.org/logrusorgru/aurora)
[![Coverage Status](https://coveralls.io/repos/logrusorgru/aurora/badge.svg?branch=master)](https://coveralls.io/r/logrusorgru/aurora?branch=master)
[![GoReportCard](https://goreportcard.com/badge/logrusorgru/aurora)](https://goreportcard.com/report/logrusorgru/aurora)
[![Gitter](https://img.shields.io/badge/chat-on_gitter-46bc99.svg?logo=data:image%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIGhlaWdodD0iMTQiIHdpZHRoPSIxNCI%2BPGcgZmlsbD0iI2ZmZiI%2BPHJlY3QgeD0iMCIgeT0iMyIgd2lkdGg9IjEiIGhlaWdodD0iNSIvPjxyZWN0IHg9IjIiIHk9IjQiIHdpZHRoPSIxIiBoZWlnaHQ9IjciLz48cmVjdCB4PSI0IiB5PSI0IiB3aWR0aD0iMSIgaGVpZ2h0PSI3Ii8%2BPHJlY3QgeD0iNiIgeT0iNCIgd2lkdGg9IjEiIGhlaWdodD0iNCIvPjwvZz48L3N2Zz4%3D&logoWidth=10)](https://gitter.im/logrusorgru/aurora)

Ultimate ANSI colors for Golang. The package supports Printf/Sprintf etc.


![aurora logo](https://github.com/logrusorgru/aurora/blob/master/gopher_aurora.png)

# Installation

Get
```
go get -u github.com/logrusorgru/aurora
```
Test
```
go test github.com/logrusorgru/aurora
```

# Usage

### Simple

```go
package main

import (
	"fmt"

	. "github.com/logrusorgru/aurora"
)

func main() {
	fmt.Println("Hello,", Magenta("Aurora"))
	fmt.Println(Bold(Cyan("Cya!")))
}

```

![simple png](https://github.com/logrusorgru/aurora/blob/master/simple.png)

### Printf

```go
package main

import (
	"fmt"

	. "github.com/logrusorgru/aurora"
)

func main() {
	fmt.Printf("Got it %d times\n", Green(1240))
	fmt.Printf("PI is %+1.2e\n", Cyan(3.14))
}

```

![printf png](https://github.com/logrusorgru/aurora/blob/master/printf.png)

### aurora.Sprintf

```go
package main

import (
	"fmt"

	. "github.com/logrusorgru/aurora"
)

func main() {
	fmt.Println(Sprintf(Magenta("Got it %d times"), Green(1240)))
}

```

![sprintf png](https://github.com/logrusorgru/aurora/blob/master/sprintf.png)

### Enable/Disable colors

```go
package main

import (
	"fmt"
	"flag"

	"github.com/logrusorgru/aurora"
)

// colorizer
var au aurora.Aurora

var colors = flag.Bool("colors", false, "enable or disable colors")

func init() {
	flag.Parse()
	au = aurora.NewAurora(*colors)
}

func main() {
	// use colorizer
	fmt.Println(au.Green("Hello"))
}

```
Without flags: 
![disable png](https://github.com/logrusorgru/aurora/blob/master/disable.png)
  
With `-colors` flag:
![enable png](https://github.com/logrusorgru/aurora/blob/master/enable.png)

# Chains

The following samples are equal

```go
x := BgMagenta(Bold(Red("x")))
```

```go
x := Red("x").Bold().BgMagenta()
```

The second is more readable

# Colorize

There is `Colorize` function that allows to choose some colors and
format from a side

```go

func getColors() Color {
	// some stuff that returns appropriate colors and format
}

// [...]

func main() {
	fmt.Println(Colorize("Greeting", getColors()))
}

```
Less complicated example

```go
x := Colorize("Greeting", GreenFg|GrayBg|BoldFm)
```

Unlike other color functions and methods (such as Red/BgBlue etc)
a `Colorize` clears previous colors

```go
x := Red("x").Colorize(BgGreen) // will be with green background only
```


# Supported colors & formats

- background and foreground colors
  + black
  + red
  + green
  + brown
  + blue
  + magenta
  + cyan
  + gray
- formats
  + bold
  + inversed

![linux png](https://github.com/logrusorgru/aurora/blob/master/linux_colors.png)  
![white png](https://github.com/logrusorgru/aurora/blob/master/white.png)

# Limitations

There is no way to represent `%T` and `%p` with colors using
a standard approach

```go
package main

import (
	"fmt"

	. "github.com/logrusorgru/aurora"
)

func main() {
	r := Red("red")
	var i int
	fmt.Printf("%T %p\n", r, Green(&i))
}
```

Output will be without colors

```
aurora.value %!p(aurora.value={0xc42000a310 768 0})
```

The obvious workaround is `Red(fmt.Sprintf("%T", some))`

### Licensing

Copyright &copy; 2016-2107 Konstantin Ivanov <kostyarin.ivanov@gmail.com>  
This work is free. It comes without any warranty, to the extent permitted
by applicable law.  You can redistribute it and/or modify it under the
terms of the Do What The Fuck You Want To Public License, Version 2,
as published by Sam Hocevar. See the LICENSE file for more details.


