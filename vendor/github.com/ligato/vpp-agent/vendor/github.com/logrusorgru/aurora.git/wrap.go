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

// Colorize wraps given value into Value with
// given colors. For example
//
//    s := Colorize("some", BlueFg|GreenBg|BoldFm)
//
// returns a Value with blue foreground, green
// background and bold. Unlike functions like
// Red/BgBlue/Bold etc. This function clears
// all previous colors. Thus
//
//    s := Colorize(Red("some"), BgBlue)
//
// clears red color from value
func Colorize(arg interface{}, color Color) Value {
	if val, ok := arg.(value); ok {
		val.color = color
		return val
	}
	return value{arg, color, 0}
}

//
// Foreground colors
//

// Black converts argument into formated/colorized value
func Black(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskFg)) | BlackFg
		return val
	}
	return value{arg, BlackFg, 0}
}

// Red converts argument into formated/colorized value
func Red(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskFg)) | RedFg
		return val
	}
	return value{arg, RedFg, 0}
}

// Green converts argument into formated/colorized value
func Green(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskFg)) | GreenFg
		return val
	}
	return value{arg, GreenFg, 0}
}

// Brown converts argument into formated/colorized value
func Brown(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskFg)) | BrownFg
		return val
	}
	return value{arg, BrownFg, 0}
}

// Blue converts argument into formated/colorized value
func Blue(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskFg)) | BlueFg
		return val
	}
	return value{arg, BlueFg, 0}
}

// Magenta converts argument into formated/colorized value
func Magenta(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskFg)) | MagentaFg
		return val
	}
	return value{arg, MagentaFg, 0}
}

// Cyan converts argument into formated/colorized value
func Cyan(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskFg)) | CyanFg
		return val
	}
	return value{arg, CyanFg, 0}
}

// Gray converts argument into formated/colorized value
func Gray(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskFg)) | GrayFg
		return val
	}
	return value{arg, GrayFg, 0}
}

//
// Background colors
//

// BgBlack converts argument into formated/colorized value
func BgBlack(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskBg)) | BlackBg
		return val
	}
	return value{arg, BlackBg, 0}
}

// BgRed converts argument into formated/colorized value
func BgRed(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskBg)) | RedBg
		return val
	}
	return value{arg, RedBg, 0}
}

// BgGreen converts argument into formated/colorized value
func BgGreen(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskBg)) | GreenBg
		return val
	}
	return value{arg, GreenBg, 0}
}

// BgBrown converts argument into formated/colorized value
func BgBrown(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskBg)) | BrownBg
		return val
	}
	return value{arg, BrownBg, 0}
}

// BgBlue converts argument into formated/colorized value
func BgBlue(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskBg)) | BlueBg
		return val
	}
	return value{arg, BlueBg, 0}
}

// BgMagenta converts argument into formated/colorized value
func BgMagenta(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskBg)) | MagentaBg
		return val
	}
	return value{arg, MagentaBg, 0}
}

// BgCyan converts argument into formated/colorized value
func BgCyan(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskBg)) | CyanBg
		return val
	}
	return value{arg, CyanBg, 0}
}

// BgGray converts argument into formated/colorized value
func BgGray(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color = (val.color & (^maskBg)) | GrayBg
		return val
	}
	return value{arg, GrayBg, 0}
}

// Bold converts argument into formated/colorized value
func Bold(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color |= BoldFm
		return val
	}
	return value{arg, BoldFm, 0}
}

// Inverse converts argument into formated/colorized value
func Inverse(arg interface{}) Value {
	if val, ok := arg.(value); ok {
		val.color |= InverseFm
		return val
	}
	return value{arg, InverseFm, 0}
}
