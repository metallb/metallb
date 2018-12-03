package main

import "strings"

// Package represents collection of objects parsed from VPP binary API JSON data
type Package struct {
	APIVersion string
	Enums      []Enum
	Unions     []Union
	Types      []Type
	Messages   []Message
	Services   []Service
	RefMap     map[string]string
}

// MessageType represents the type of a VPP message
type MessageType int

const (
	requestMessage MessageType = iota // VPP request message
	replyMessage                      // VPP reply message
	eventMessage                      // VPP event message
	otherMessage                      // other VPP message
)

// Message represents VPP binary API message
type Message struct {
	Name   string
	CRC    string
	Fields []Field
}

// Type represents VPP binary API type
type Type struct {
	Name   string
	CRC    string
	Fields []Field
}

// Union represents VPP binary API union
type Union struct {
	Name   string
	CRC    string
	Fields []Field
}

// Field represents VPP binary API object field
type Field struct {
	Name     string
	Type     string
	Length   int
	SizeFrom string
}

func (f *Field) IsArray() bool {
	return f.Length > 0 || f.SizeFrom != ""
}

// Enum represents VPP binary API enum
type Enum struct {
	Name    string
	Type    string
	Entries []EnumEntry
}

// EnumEntry represents VPP binary API enum entry
type EnumEntry struct {
	Name  string
	Value interface{}
}

// Service represents VPP binary API service
type Service struct {
	Name        string
	RequestType string
	ReplyType   string
	Stream      bool
	Events      []string
}

func (s Service) MethodName() string {
	reqTyp := camelCaseName(s.RequestType)

	// method name is same as parameter type name by default
	method := reqTyp
	if s.Stream {
		// use Dump as prefix instead of suffix for stream services
		if m := strings.TrimSuffix(method, "Dump"); method != m {
			method = "Dump" + m
		}
	}

	return method
}

func (s Service) IsDumpService() bool {
	return s.Stream
}

func (s Service) IsEventService() bool {
	return len(s.Events) > 0
}

func (s Service) IsRequestService() bool {
	// some binapi messages might have `null` reply (for example: memclnt)
	return s.ReplyType != "" && s.ReplyType != "null" // not null
}

func getSizeOfType(typ *Type) (size int) {
	for _, field := range typ.Fields {
		if n := getBinapiTypeSize(field.Type); n > 0 {
			if field.Length > 0 {
				size += n * field.Length
			} else {
				size += n
			}
		}
	}
	return size
}
