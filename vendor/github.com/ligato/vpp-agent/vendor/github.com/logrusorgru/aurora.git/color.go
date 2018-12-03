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

// A Color type is a color. It can contains
// boldness, "inverseness", one background color
// and one foreground color
type Color int

// special formats
const (
	BoldFm    Color = 1 << iota // bold
	InverseFm                   // inverse
)

// foreground
const (
	BlackFg   Color = (1 + iota) << 8 // black
	RedFg                             // red
	GreenFg                           // green
	BrownFg                           // brown
	BlueFg                            // blue
	MagentaFg                         // magenta
	CyanFg                            // cyan
	GrayFg                            // gray

	maskFg = (BlackFg | RedFg | GreenFg | BrownFg | BlueFg | MagentaFg |
		CyanFg | GrayFg)
)

// background
const (
	BlackBg   Color = (1 + iota) << 16 // black
	RedBg                              // red
	GreenBg                            // green
	BrownBg                            // brown
	BlueBg                             // blue
	MagentaBg                          // magenta
	CyanBg                             // cyan
	GrayBg                             // gray

	maskBg = (BlackBg | RedBg | GreenBg | BrownBg | BlueBg | MagentaBg |
		CyanBg | GrayBg)
)

// IsValid returns true if a color looks like valid
func (c Color) IsValid() bool {
	return c&(BoldFm|InverseFm|maskFg|maskBg) != 0 || c == 0
}

const (
	availFlags = "-+# 0"
	esc        = "\033["
	clear      = esc + "0m"
)

// Nos returns string like 1;7;31;45. It may be an empty string for
// empty or invalid color
func (c Color) Nos() string {
	if c.IsValid() && c != 0 {
		return string(c.appendNos(make([]byte, 0, 9)))
	}
	return ""
}

func (c Color) appendNos(bs []byte) []byte {
	var semicolon bool
	if c&BoldFm != 0 {
		semicolon = true
		bs = append(bs, '1')
	}
	if c&InverseFm != 0 {
		if semicolon {
			bs = append(bs, ';')
		} else {
			semicolon = true
		}
		bs = append(bs, '7')
	}
	if c&maskFg != 0 {
		if semicolon {
			bs = append(bs, ';')
		} else {
			semicolon = true
		}
		bs = append(bs, '3', '0'+byte((c>>8)&0xff)-1)
	}
	if c&maskBg != 0 {
		if semicolon {
			bs = append(bs, ';')
		}
		bs = append(bs, '4', '0'+byte((c>>16)&0xff)-1)
	}
	return bs
}
