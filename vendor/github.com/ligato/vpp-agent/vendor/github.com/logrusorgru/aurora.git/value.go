//
// Copyright (c) 2016 Konstanin Ivanov <kostyarin.ivanov@gmail.com>.
// All rights reserved. This program is free software. It comes without
// any warranty, to the extent permitted by applicable law. You can
// redistribute it and/or modify it under the terms of the Do What
// The Fuck You Want To Public License, Version 2, as published by
// Sam Hocevar. See LICENSE file for more details or see below.
//

//
//        DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE
//                    Version 2, December 2004
//
// Copyright (C) 2004 Sam Hocevar <sam@hocevar.net>
//
// Everyone is permitted to copy and distribute verbatim or modified
// copies of this license document, and changing it is allowed as long
// as the name is changed.
//
//            DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE
//   TERMS AND CONDITIONS FOR COPYING, DISTRIBUTION AND MODIFICATION
//
//  0. You just DO WHAT THE FUCK YOU WANT TO.
//

package aurora

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

// A Value represents any printable value
// with it's color
type Value interface {
	// String returns string with colors. If there are any color
	// or format the string will be terminated with \033[0m
	fmt.Stringer
	// Format implements fmt.Formater interface
	fmt.Formatter
	// Color returns value's color
	Color() Color
	// Value returns value's value (welcome to the tautology club)
	Value() interface{}
	// Bleach returns copy of orignal value without colors
	Bleach() Value
	//
	tail() Color
	setTail(Color) Value

	Black() Value     // change foreground color to black
	Red() Value       // change foreground color to red
	Green() Value     // change foreground color to green
	Brown() Value     // change foreground color to brown
	Blue() Value      // change foreground color to blue
	Magenta() Value   // change foreground color to magenta
	Cyan() Value      // change foreground color to cyan
	Gray() Value      // change foreground color to gray
	BgBlack() Value   // change background color to black
	BgRed() Value     // change background color to red
	BgGreen() Value   // change background color to green
	BgBrown() Value   // change background color to brown
	BgBlue() Value    // change background color to blue
	BgMagenta() Value // change background color to magenta
	BgCyan() Value    // change background color to cyan
	BgGray() Value    // change background color to gray
	Bold() Value      // change format to bold
	Inverse() Value   // change format to inversed
}

// Value without colors

type valueClear struct {
	value interface{}
}

func (v valueClear) String() string      { return fmt.Sprint(v.value) }
func (v valueClear) Color() Color        { return 0 }
func (v valueClear) Bleach() Value       { return v }
func (v valueClear) Value() interface{}  { return v.value }
func (v valueClear) tail() Color         { return 0 }
func (v valueClear) setTail(Color) Value { return v }

func (v valueClear) Black() Value     { return v }
func (v valueClear) Red() Value       { return v }
func (v valueClear) Green() Value     { return v }
func (v valueClear) Brown() Value     { return v }
func (v valueClear) Blue() Value      { return v }
func (v valueClear) Magenta() Value   { return v }
func (v valueClear) Cyan() Value      { return v }
func (v valueClear) Gray() Value      { return v }
func (v valueClear) BgBlack() Value   { return v }
func (v valueClear) BgRed() Value     { return v }
func (v valueClear) BgGreen() Value   { return v }
func (v valueClear) BgBrown() Value   { return v }
func (v valueClear) BgBlue() Value    { return v }
func (v valueClear) BgMagenta() Value { return v }
func (v valueClear) BgCyan() Value    { return v }
func (v valueClear) BgGray() Value    { return v }
func (v valueClear) Bold() Value      { return v }
func (v valueClear) Inverse() Value   { return v }

func (v valueClear) Format(s fmt.State, verb rune) {
	// it's enough for many cases (%-+020.10f)
	// %          - 1
	// availFlags - 3 (5)
	// width      - 2
	// prec       - 3 (.23)
	// verb       - 1
	// --------------
	//             10
	format := make([]byte, 1, 10)
	format[0] = '%'
	var f byte
	for i := 0; i < len(availFlags); i++ {
		if f = availFlags[i]; s.Flag(int(f)) {
			format = append(format, f)
		}
	}
	var width, prec int
	var ok bool
	if width, ok = s.Width(); ok {
		format = strconv.AppendInt(format, int64(width), 10)
	}
	if prec, ok = s.Precision(); ok {
		format = append(format, '.')
		format = strconv.AppendInt(format, int64(prec), 10)
	}
	if verb > utf8.RuneSelf {
		format = append(format, string(verb)...)
	} else {
		format = append(format, byte(verb))
	}
	fmt.Fprintf(s, string(format), v.value)
}

// Value within colors

type value struct {
	value     interface{}
	color     Color
	tailColor Color
}

func (v value) String() string {
	if v.color != 0 && v.color.IsValid() {
		if v.tailColor != 0 && v.tailColor.IsValid() {
			return esc + v.color.Nos() + "m" + fmt.Sprint(v.value) + clear +
				esc + v.tailColor.Nos() + "m"
		}
		return esc + v.color.Nos() + "m" + fmt.Sprint(v.value) + clear
	}
	return fmt.Sprint(v.value)
}

func (v value) Color() Color { return v.color }

func (v value) Bleach() Value {
	v.color, v.tailColor = 0, 0
	return v
}

func (v value) tail() Color { return v.tailColor }
func (v value) setTail(t Color) Value {
	v.tailColor = t
	return v
}

func (v value) Value() interface{} { return v.value }

func (v value) Format(s fmt.State, verb rune) {
	// it's enough for many cases (%-+020.10f)
	// %          - 1
	// availFlags - 3 (5)
	// width      - 2
	// prec       - 3 (.23)
	// verb       - 1
	// --------------
	//             10
	// +
	// \033[1;7;31;45m   - 12 x2 (+possible tailColor)
	// \033[0m           - 4
	// --------------------------
	//                    38
	format := make([]byte, 0, 38)
	var colors bool
	if v.color != 0 && v.color.IsValid() {
		colors = true
		format = append(format, esc...)
		format = v.color.appendNos(format)
		format = append(format, 'm')
	}
	format = append(format, '%')
	var f byte
	for i := 0; i < len(availFlags); i++ {
		if f = availFlags[i]; s.Flag(int(f)) {
			format = append(format, f)
		}
	}
	var width, prec int
	var ok bool
	if width, ok = s.Width(); ok {
		format = strconv.AppendInt(format, int64(width), 10)
	}
	if prec, ok = s.Precision(); ok {
		format = append(format, '.')
		format = strconv.AppendInt(format, int64(prec), 10)
	}
	if verb > utf8.RuneSelf {
		format = append(format, string(verb)...)
	} else {
		format = append(format, byte(verb))
	}
	if colors {
		format = append(format, clear...)
		if v.tailColor != 0 && v.tailColor.IsValid() { // next format
			format = append(format, esc...)
			format = v.tailColor.appendNos(format)
			format = append(format, 'm')
		}
	}
	fmt.Fprintf(s, string(format), v.value)
}

func (v value) Black() Value {
	v.color = (v.color & (^maskFg)) | BlackFg
	return v
}

func (v value) Red() Value {
	v.color = (v.color & (^maskFg)) | RedFg
	return v
}

func (v value) Green() Value {
	v.color = (v.color & (^maskFg)) | GreenFg
	return v
}

func (v value) Brown() Value {
	v.color = (v.color & (^maskFg)) | BrownFg
	return v
}

func (v value) Blue() Value {
	v.color = (v.color & (^maskFg)) | BlueFg
	return v
}

func (v value) Magenta() Value {
	v.color = (v.color & (^maskFg)) | MagentaFg
	return v
}

func (v value) Cyan() Value {
	v.color = (v.color & (^maskFg)) | CyanFg
	return v
}

func (v value) Gray() Value {
	v.color = (v.color & (^maskFg)) | GrayFg
	return v
}

func (v value) BgBlack() Value {
	v.color = (v.color & (^maskBg)) | BlackBg
	return v
}

func (v value) BgRed() Value {
	v.color = (v.color & (^maskBg)) | RedBg
	return v
}

func (v value) BgGreen() Value {
	v.color = (v.color & (^maskBg)) | GreenBg
	return v
}

func (v value) BgBrown() Value {
	v.color = (v.color & (^maskBg)) | BrownBg
	return v
}

func (v value) BgBlue() Value {
	v.color = (v.color & (^maskBg)) | BlueBg
	return v
}

func (v value) BgMagenta() Value {
	v.color = (v.color & (^maskBg)) | MagentaBg
	return v
}

func (v value) BgCyan() Value {
	v.color = (v.color & (^maskBg)) | CyanBg
	return v
}

func (v value) BgGray() Value {
	v.color = (v.color & (^maskBg)) | GrayBg
	return v
}

func (v value) Bold() Value {
	v.color |= BoldFm
	return v
}

func (v value) Inverse() Value {
	v.color |= InverseFm
	return v
}
