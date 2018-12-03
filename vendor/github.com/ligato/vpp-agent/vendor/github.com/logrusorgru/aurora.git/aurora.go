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

// Package aurora implements ANSI-colors
package aurora

import (
	"fmt"
)

// An Aurora implements colorizer interface.
// It also can be non-colorizer
type Aurora interface {
	Black(arg interface{}) Value
	Red(arg interface{}) Value
	Green(arg interface{}) Value
	Brown(arg interface{}) Value
	Blue(arg interface{}) Value
	Magenta(arg interface{}) Value
	Cyan(arg interface{}) Value
	Gray(arg interface{}) Value
	BgBlack(arg interface{}) Value
	BgRed(arg interface{}) Value
	BgGreen(arg interface{}) Value
	BgBrown(arg interface{}) Value
	BgBlue(arg interface{}) Value
	BgMagenta(arg interface{}) Value
	BgCyan(arg interface{}) Value
	BgGray(arg interface{}) Value
	Bold(arg interface{}) Value
	Inverse(arg interface{}) Value
	Colorize(arg interface{}, color Color) Value
	Sprintf(format interface{}, args ...interface{}) string
}

// NewAurora returns a new Aurora interface that
// will support or not support colors depending
// the enableColors argument
func NewAurora(enableColors bool) Aurora {
	if enableColors {
		return aurora{}
	}
	return auroraClear{}
}

// no colors

type auroraClear struct{}

func (auroraClear) Black(arg interface{}) Value     { return valueClear{arg} }
func (auroraClear) Red(arg interface{}) Value       { return valueClear{arg} }
func (auroraClear) Green(arg interface{}) Value     { return valueClear{arg} }
func (auroraClear) Brown(arg interface{}) Value     { return valueClear{arg} }
func (auroraClear) Blue(arg interface{}) Value      { return valueClear{arg} }
func (auroraClear) Magenta(arg interface{}) Value   { return valueClear{arg} }
func (auroraClear) Cyan(arg interface{}) Value      { return valueClear{arg} }
func (auroraClear) Gray(arg interface{}) Value      { return valueClear{arg} }
func (auroraClear) BgBlack(arg interface{}) Value   { return valueClear{arg} }
func (auroraClear) BgRed(arg interface{}) Value     { return valueClear{arg} }
func (auroraClear) BgGreen(arg interface{}) Value   { return valueClear{arg} }
func (auroraClear) BgBrown(arg interface{}) Value   { return valueClear{arg} }
func (auroraClear) BgBlue(arg interface{}) Value    { return valueClear{arg} }
func (auroraClear) BgMagenta(arg interface{}) Value { return valueClear{arg} }
func (auroraClear) BgCyan(arg interface{}) Value    { return valueClear{arg} }
func (auroraClear) BgGray(arg interface{}) Value    { return valueClear{arg} }
func (auroraClear) Bold(arg interface{}) Value      { return valueClear{arg} }
func (auroraClear) Inverse(arg interface{}) Value   { return valueClear{arg} }

func (auroraClear) Colorize(arg interface{}, color Color) Value {
	return valueClear{arg}
}

func (auroraClear) Sprintf(format interface{}, args ...interface{}) string {
	if str, ok := format.(string); ok {
		return fmt.Sprintf(str, args...)
	}
	return fmt.Sprintf(fmt.Sprint(format), args...)
}

// colorized

type aurora struct{}

func (aurora) Black(arg interface{}) Value     { return Black(arg) }
func (aurora) Red(arg interface{}) Value       { return Red(arg) }
func (aurora) Green(arg interface{}) Value     { return Green(arg) }
func (aurora) Brown(arg interface{}) Value     { return Brown(arg) }
func (aurora) Blue(arg interface{}) Value      { return Blue(arg) }
func (aurora) Magenta(arg interface{}) Value   { return Magenta(arg) }
func (aurora) Cyan(arg interface{}) Value      { return Cyan(arg) }
func (aurora) Gray(arg interface{}) Value      { return Gray(arg) }
func (aurora) BgBlack(arg interface{}) Value   { return BgBlack(arg) }
func (aurora) BgRed(arg interface{}) Value     { return BgRed(arg) }
func (aurora) BgGreen(arg interface{}) Value   { return BgGreen(arg) }
func (aurora) BgBrown(arg interface{}) Value   { return BgBrown(arg) }
func (aurora) BgBlue(arg interface{}) Value    { return BgBlue(arg) }
func (aurora) BgMagenta(arg interface{}) Value { return BgMagenta(arg) }
func (aurora) BgCyan(arg interface{}) Value    { return BgCyan(arg) }
func (aurora) BgGray(arg interface{}) Value    { return BgGray(arg) }
func (aurora) Bold(arg interface{}) Value      { return Bold(arg) }
func (aurora) Inverse(arg interface{}) Value   { return Inverse(arg) }

func (aurora) Colorize(arg interface{}, color Color) Value {
	return Colorize(arg, color)
}

func (aurora) Sprintf(format interface{}, args ...interface{}) string {
	return Sprintf(format, args...)
}
