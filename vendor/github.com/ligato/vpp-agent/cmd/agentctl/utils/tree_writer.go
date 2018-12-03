// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"bytes"
	"fmt"
	"strconv"
)

// PrintLine represents one line in the tree output.
type PrintLine struct {
	line    string
	lnLevel int
	subtree []PrintLine
}

type lineBuf []PrintLine

// TreeWriter is an implementation of the TreePrinter interface.
type TreeWriter struct {
	writeBuf []byte
	level    int
	lineBuf  []PrintLine

	spaces     int
	firstDash  string
	middleDash string
	lastDash   string
}

// NewTreeWriter returns a reference to a newly created TreeWriter
// instance. Parameters passed into this function determine the visual
// appearance of the tree in which the data is printed. A typical usage
// would be:
//   p := NewTreeWriter(1, "├─", "│ ", "└─")
func NewTreeWriter(spaces int, first string, middle string, last string) *TreeWriter {
	return &TreeWriter{
		writeBuf:   []byte{},
		lineBuf:    []PrintLine{},
		spaces:     spaces,
		firstDash:  first,
		middleDash: middle,
		lastDash:   last,
	}
}

// FlushTree takes the content of the finalized buffer, and formats it
// into a tree, and prints it out to stdout.
func (p *TreeWriter) FlushTree() {

	p.lineBuf = createPrintLineBuf(p.writeBuf)
	//for i, lbl := range p.lineBuf {
	//	fmt.Printf("%d: Level %d, line '%s'\n", i, lbl.lnLevel, lbl.line)
	//}

	tree, _ := createTree(1, p.lineBuf)
	stack := &PfxStack{
		Entries:    []PfxStackEntry{},
		Spaces:     p.spaces,
		FirstDash:  p.firstDash,
		MiddleDash: p.middleDash,
		LastDash:   p.lastDash,
	}
	stack.Push()
	p.renderSubtree(tree, stack)
	p.writeBuf = []byte{}
}

// createPrintLineBuf creates a new buffer of PrintLine structs
// that are then used to create a PrintLine tree which is used to
// render the tree. The function translates the content of a raw
// write buffer into a flat buffer of printLines.
//
// The function expects that each line in the raw write buffer contains
// PrintLine level information - each line in the write buffer is
// expected to have the format '<level>^@<content-of-the-line>, where
// '^@' is the separator.
func createPrintLineBuf(byteBuf []byte) []PrintLine {
	lines := bytes.Split(bytes.TrimSpace(byteBuf), []byte{10})

	printLineBuf := make([]PrintLine, 0, len(lines)+1)
	for _, line := range lines {
		lbl := PrintLine{}
		if len(line) == 0 {
			lbl.line = string(line)
		} else {
			aux := bytes.Split(line, []byte{'^', '@'})
			lbl.lnLevel, _ = strconv.Atoi(string(aux[0]))
			lbl.line = string(bytes.TrimSpace(aux[1]))
		}
		printLineBuf = append(printLineBuf, lbl)
	}
	for i, lbl := range printLineBuf {
		if len(lbl.line) == 0 {
			lbl.lnLevel = printLineBuf[i+1].lnLevel
			printLineBuf[i] = lbl
		}
	}
	return printLineBuf
}

// renderSubtree is used to recursively render the tree.
func (p *TreeWriter) renderSubtree(tree []PrintLine, stack *PfxStack) {
	for i, pl := range tree {
		if i == len(tree)-1 {
			stack.SetLast()
		}

		var pp string
		if pl.line == "" {
			pp = stack.setTopPfxStackEntry(stack.GetPreamble(stack.MiddleDash))
		} else {
			pp = stack.getTopPfxStackEntry()
		}
		//fmt.Printf("%2d of %2d: level %d, line: '%s %s'\n",
		// 		i, len(tree), pl.lnLevel, stack.GetPrefix(), pl.line)
		fmt.Printf("%s %s\n", stack.GetPrefix(), pl.line)
		stack.setTopPfxStackEntry(pp)

		if len(pl.subtree) > 0 {
			stack.Push()
			p.renderSubtree(pl.subtree, stack)
			stack.Pop()
		}

	}
}

// createTree creates a tree of PrintLine structs from a flat PrintLine
// buffer (typically created in createPrintLineBuf()).
func createTree(curLevel int, lineBuf []PrintLine) ([]PrintLine, int) {
	//fmt.Printf("--> Enter createTree: curLevel %d, lineBufLen %d, line[0]: %s\n",
	// 	curLevel, len(lineBuf), lineBuf[0].line)
	res := []PrintLine{}
	processed := 0
	lb := lineBuf

Loop:
	for len(lb) > 0 {
		// fmt.Printf("   lb[0]: lnLevel %d, line '%s'\n", lb[0].lnLevel, lb[0].line)
		if lb[0].lnLevel < curLevel {
			break Loop
		} else if lb[0].lnLevel == curLevel {
			res = append(res, lb[0])
			processed++
			lb = lb[1:]
		} else {
			subtree, p := createTree(lb[0].lnLevel, lb)
			res[len(res)-1].subtree = subtree
			processed += p
			lb = lb[p:]
		}
	}
	//fmt.Printf("<-- Return createTree: curLevel %d, lineBufLen %d, line[0]: %s\n",
	//	curLevel, len(lineBuf), lineBuf[0].line)
	return res, processed
}

// Write is an override of io.Write - it only collects the data
// to be written in a holding buffer for later printing in the
// FlushTable() function.
func (p *TreeWriter) Write(b []byte) (n int, err error) {
	// fmt.Printf("'%s'", b)
	p.writeBuf = append(p.writeBuf[:], b[:]...)
	return len(b), nil
}
