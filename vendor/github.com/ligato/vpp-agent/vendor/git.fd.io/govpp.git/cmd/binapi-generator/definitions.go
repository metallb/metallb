// Copyright (c) 2018 Cisco and/or its affiliates.
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

package main

import (
	"strconv"
	"strings"
	"unicode"
)

func getBinapiTypeSize(binapiType string) int {
	if _, ok := binapiTypes[binapiType]; ok {
		b, err := strconv.Atoi(strings.TrimLeft(binapiType, "uif"))
		if err == nil {
			return b / 8
		}
	}
	return -1
}

// binapiTypes is a set of types used VPP binary API for translation to Go types
var binapiTypes = map[string]string{
	"u8":  "uint8",
	"i8":  "int8",
	"u16": "uint16",
	"i16": "int16",
	"u32": "uint32",
	"i32": "int32",
	"u64": "uint64",
	"i64": "int64",
	"f64": "float64",
}

func usesInitialism(s string) string {
	if u := strings.ToUpper(s); commonInitialisms[u] {
		return u
	} else if su, ok := specialInitialisms[u]; ok {
		return su
	}
	return ""
}

// commonInitialisms is a set of common initialisms that need to stay in upper case.
var commonInitialisms = map[string]bool{
	"ACL": true,
	"API": true,
	//"ASCII": true, // there are only two use cases for ASCII which already have initialism before and after
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"DHCP":  true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"ICMP":  true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"PID":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SQL":   true,
	"SSH":   true,
	"TCP":   true,
	"TLS":   true,
	"TTL":   true,
	"UDP":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"VPN":   true,
	"XML":   true,
	"XMPP":  true,
	"XSRF":  true,
	"XSS":   true,
}

// specialInitialisms is a set of special initialisms that need part to stay in upper case.
var specialInitialisms = map[string]string{
	"IPV": "IPv",
	//"IPV4": "IPv4",
	//"IPV6": "IPv6",
}

// camelCaseName returns correct name identifier (camelCase).
func camelCaseName(name string) (should string) {
	name = strings.Title(name)

	// Fast path for simple cases: "_" and all lowercase.
	if name == "_" {
		return name
	}
	allLower := true
	for _, r := range name {
		if !unicode.IsLower(r) {
			allLower = false
			break
		}
	}
	if allLower {
		return name
	}

	// Split camelCase at any lower->upper transition, and split on underscores.
	// Check each word for common initialisms.
	runes := []rune(name)
	w, i := 0, 0 // index of start of word, scan
	for i+1 <= len(runes) {
		eow := false // whether we hit the end of a word
		if i+1 == len(runes) {
			eow = true
		} else if runes[i+1] == '_' {
			// underscore; shift the remainder forward over any run of underscores
			eow = true
			n := 1
			for i+n+1 < len(runes) && runes[i+n+1] == '_' {
				n++
			}

			// Leave at most one underscore if the underscore is between two digits
			if i+n+1 < len(runes) && unicode.IsDigit(runes[i]) && unicode.IsDigit(runes[i+n+1]) {
				n--
			}

			copy(runes[i+1:], runes[i+n+1:])
			runes = runes[:len(runes)-n]
		} else if unicode.IsLower(runes[i]) && !unicode.IsLower(runes[i+1]) {
			// lower->non-lower
			eow = true
		}
		i++
		if !eow {
			continue
		}

		// [w,i) is a word.
		word := string(runes[w:i])
		if u := usesInitialism(word); u != "" {
			// Keep consistent case, which is lowercase only at the start.
			if w == 0 && unicode.IsLower(runes[w]) {
				u = strings.ToLower(u)
			}
			// All the common initialisms are ASCII,
			// so we can replace the bytes exactly.
			copy(runes[w:], []rune(u))
		} else if w > 0 && strings.ToLower(word) == word {
			// already all lowercase, and not the first word, so uppercase the first character.
			runes[w] = unicode.ToUpper(runes[w])
		}
		w = i
	}
	return string(runes)
}
