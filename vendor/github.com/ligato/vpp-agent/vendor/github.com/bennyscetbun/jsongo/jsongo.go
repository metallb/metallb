// Copyright 2014 Benny Scetbun. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package Jsongo is a simple library to help you build Json without static struct
//
// Source code and project home:
// https://github.com/benny-deluxe/jsongo
//

package jsongo

import (
	"encoding/json"
	"errors"
	"reflect"
	//"fmt"
)

//ErrorKeyAlreadyExist error if a key already exist in current JSONNode
var ErrorKeyAlreadyExist = errors.New("jsongo key already exist")

//ErrorMultipleType error if a JSONNode already got a different type of value
var ErrorMultipleType = errors.New("jsongo this node is already set to a different jsonNodeType")

//ErrorArrayNegativeValue error if you ask for a negative index in an array
var ErrorArrayNegativeValue = errors.New("jsongo negative index for array")

//ErrorArrayNegativeValue error if you ask for a negative index in an array
var ErrorAtUnsupportedType = errors.New("jsongo Unsupported Type as At argument")

//ErrorRetrieveUserValue error if you ask the value of a node that is not a value node
var ErrorRetrieveUserValue = errors.New("jsongo Cannot retrieve node's value which is not of type value")

//ErrorTypeUnmarshaling error if you try to unmarshal something in the wrong type
var ErrorTypeUnmarshaling = errors.New("jsongo Wrong type when Unmarshaling")

//ErrorUnknowType error if you try to use an unknow JSONNodeType
var ErrorUnknowType = errors.New("jsongo Unknow JSONNodeType")

//ErrorValNotPointer error if you try to use Val without a valid pointer
var ErrorValNotPointer = errors.New("jsongo: Val: arguments must be a pointer and not nil")

//ErrorGetKeys error if you try to get the keys from a JSONNode that isnt a TypeMap or a TypeArray
var ErrorGetKeys = errors.New("jsongo: GetKeys: JSONNode is not a TypeMap or TypeArray")

//ErrorDeleteKey error if you try to call DelKey on a JSONNode that isnt a TypeMap
var ErrorDeleteKey = errors.New("jsongo: DelKey: This JSONNode is not a TypeMap")

//ErrorCopyType error if you try to call Copy on a JSONNode that isnt a TypeUndefined
var ErrorCopyType = errors.New("jsongo: Copy: This JSONNode is not a TypeUndefined")

//JSONNode Datastructure to build and maintain Nodes
type JSONNode struct {
	m          map[string]*JSONNode
	a          []JSONNode
	v          interface{}
	vChanged   bool         //True if we changed the type of the value
	t          JSONNodeType //Type of that JSONNode 0: Not defined, 1: map, 2: array, 3: value
	dontExpand bool         //dont expand while Unmarshal
}

//JSONNodeType is used to set, check and get the inner type of a JSONNode
type JSONNodeType uint

const (
	//TypeUndefined is set by default for empty JSONNode
	TypeUndefined JSONNodeType = iota
	//TypeMap is set when a JSONNode is a Map
	TypeMap
	//TypeArray is set when a JSONNode is an Array
	TypeArray
	//TypeValue is set when a JSONNode is a Value Node
	TypeValue
	//typeError help us detect errors
	typeError
)

//At helps you move through your node by building them on the fly
//
//val can be string or int only
//
//strings are keys for TypeMap
//
//ints are index in TypeArray (it will make array grow on the fly, so you should start to populate with the biggest index first)*
func (that *JSONNode) At(val ...interface{}) *JSONNode {
	if len(val) == 0 {
		return that
	}
	switch vv := val[0].(type) {
	case string:
		return that.atMap(vv, val[1:]...)
	case int:
		return that.atArray(vv, val[1:]...)
	}
	panic(ErrorAtUnsupportedType)
}

//atMap return the JSONNode in current map
func (that *JSONNode) atMap(key string, val ...interface{}) *JSONNode {
	if that.t != TypeUndefined && that.t != TypeMap {
		panic(ErrorMultipleType)
	}
	if that.m == nil {
		that.m = make(map[string]*JSONNode)
		that.t = TypeMap
	}
	if next, ok := that.m[key]; ok {
		return next.At(val...)
	}
	that.m[key] = new(JSONNode)
	return that.m[key].At(val...)
}

//atArray return the JSONNode in current TypeArray (and make it grow if necessary)
func (that *JSONNode) atArray(key int, val ...interface{}) *JSONNode {
	if that.t == TypeUndefined {
		that.t = TypeArray
	} else if that.t != TypeArray {
		panic(ErrorMultipleType)
	}
	if key < 0 {
		panic(ErrorArrayNegativeValue)
	}
	if key >= len(that.a) {
		newa := make([]JSONNode, key+1)
		for i := 0; i < len(that.a); i++ {
			newa[i] = that.a[i]
		}
		that.a = newa
	}
	return that.a[key].At(val...)
}

//Map Turn this JSONNode to a TypeMap and/or Create a new element for key if necessary and return it
func (that *JSONNode) Map(key string) *JSONNode {
	if that.t != TypeUndefined && that.t != TypeMap {
		panic(ErrorMultipleType)
	}
	if that.m == nil {
		that.m = make(map[string]*JSONNode)
		that.t = TypeMap
	}
	if _, ok := that.m[key]; ok {
		return that.m[key]
	}
	that.m[key] = &JSONNode{}
	return that.m[key]
}

//Array Turn this JSONNode to a TypeArray and/or set the array size (reducing size will make you loose data)
func (that *JSONNode) Array(size int) *[]JSONNode {
	if that.t == TypeUndefined {
		that.t = TypeArray
	} else if that.t != TypeArray {
		panic(ErrorMultipleType)
	}
	if size < 0 {
		panic(ErrorArrayNegativeValue)
	}
	var min int
	if size < len(that.a) {
		min = size
	} else {
		min = len(that.a)
	}
	newa := make([]JSONNode, size)
	for i := 0; i < min; i++ {
		newa[i] = that.a[i]
	}
	that.a = newa
	return &(that.a)
}

//Val Turn this JSONNode to Value type and/or set that value to val
func (that *JSONNode) Val(val interface{}) {
	if that.t == TypeUndefined {
		that.t = TypeValue
	} else if that.t != TypeValue {
		panic(ErrorMultipleType)
	}
	rt := reflect.TypeOf(val)
	var finalval interface{}
	if val == nil {
		finalval = &val
		that.vChanged = true
	} else if rt.Kind() != reflect.Ptr {
		rv := reflect.ValueOf(val)
		var tmp reflect.Value
		if rv.CanAddr() {
			tmp = rv.Addr()
		} else {
			tmp = reflect.New(rt)
			tmp.Elem().Set(rv)
		}
		finalval = tmp.Interface()
		that.vChanged = true
	} else {
		finalval = val
	}
	that.v = finalval
}

//Get Return value of a TypeValue as interface{}
func (that *JSONNode) Get() interface{} {
	if that.t != TypeValue {
		panic(ErrorRetrieveUserValue)
	}
	if that.vChanged {
		rv := reflect.ValueOf(that.v)
		return rv.Elem().Interface()
	}
	return that.v
}

//GetKeys Return a slice interface that represent the keys to use with the At fonction (Works only on TypeMap and TypeArray)
func (that *JSONNode) GetKeys() []interface{} {
	var ret []interface{}
	switch that.t {
	case TypeMap:
		nb := len(that.m)
		ret = make([]interface{}, nb)
		for key := range that.m {
			nb--
			ret[nb] = key
		}
	case TypeArray:
		nb := len(that.a)
		ret = make([]interface{}, nb)
		for nb > 0 {
			nb--
			ret[nb] = nb
		}
	default:
		panic(ErrorGetKeys)
	}
	return ret
}

//Len Return the length of the current Node
//
// if TypeUndefined return 0
//
// if TypeValue return 1
//
// if TypeArray return the size of the array
//
// if TypeMap return the size of the map
func (that *JSONNode) Len() int {
	var ret int
	switch that.t {
	case TypeMap:
		ret = len(that.m)
	case TypeArray:
		ret = len(that.a)
	case TypeValue:
		ret = 1
	}
	return ret
}

//SetType Is use to set the Type of a node and return the current Node you are working on
func (that *JSONNode) SetType(t JSONNodeType) *JSONNode {
	if that.t != TypeUndefined && that.t != t {
		panic(ErrorMultipleType)
	}
	if t >= typeError {
		panic(ErrorUnknowType)
	}
	that.t = t
	switch t {
	case TypeMap:
		that.m = make(map[string]*JSONNode, 0)
	case TypeArray:
		that.a = make([]JSONNode, 0)
	case TypeValue:
		that.Val(nil)
	}
	return that
}

//GetType Is use to Get the Type of a node
func (that *JSONNode) GetType() JSONNodeType {
	return that.t
}

//Copy Will set this node like the one in argument. this node must be of type TypeUndefined
//
//if deepCopy is true we will copy all the children recursively else we will share the children
//
//return the current JSONNode
func (that *JSONNode) Copy(other *JSONNode, deepCopy bool) *JSONNode {
	if that.t != TypeUndefined {
		panic(ErrorCopyType)
	}
	
	if other.t == TypeValue {
		*that = *other
	} else if other.t == TypeArray {
		if !deepCopy {
			*that = *other
		} else {
			that.Array(len(other.a))
			for i := range other.a {
				that.At(i).Copy(other.At(i), deepCopy)
			}
		}
	} else if other.t == TypeMap {
		that.SetType(other.t)
		if !deepCopy {
			for val := range other.m {
				that.m[val] = other.m[val]
			}
		} else {
			for val := range other.m {
				that.Map(val).Copy(other.At(val), deepCopy)
			}
		}
	}
	return that
}


//Unset Will unset everything in the JSONnode. All the children data will be lost
func (that *JSONNode) Unset() {
	*that = JSONNode{}
}

//DelKey will remove a key in the map.
//
//return the current JSONNode.
func (that *JSONNode) DelKey(key string) *JSONNode {
	if that.t != TypeMap {
		panic(ErrorDeleteKey)
	}
	delete(that.m, key)
	return that
}

//UnmarshalDontExpand set or not if Unmarshall will generate anything in that JSONNode and its children
//
//val: will change the expanding rules for this node
//
//- The type wont be change for any type
//
//- Array wont grow
//
//- New keys wont be added to Map
//
//- Values set to nil "*.Val(nil)*" will be turn into the type decide by Json
//
//- It will respect any current mapping and will return errors if needed
//
//recurse: if true, it will set all the children of that JSONNode with val
func (that *JSONNode) UnmarshalDontExpand(val bool, recurse bool) *JSONNode {
	that.dontExpand = val
	if recurse {
		switch that.t {
		case TypeMap:
			for k := range that.m {
				that.m[k].UnmarshalDontExpand(val, recurse)
			}
		case TypeArray:
			for k := range that.a {
				that.a[k].UnmarshalDontExpand(val, recurse)
			}
		}
	}
	return that
}

//MarshalJSON Make JSONNode a Marshaler Interface compatible
func (that *JSONNode) MarshalJSON() ([]byte, error) {
	var ret []byte
	var err error
	switch that.t {
	case TypeMap:
		ret, err = json.Marshal(that.m)
	case TypeArray:
		ret, err = json.Marshal(that.a)
	case TypeValue:
		ret, err = json.Marshal(that.v)
	default:
		ret, err = json.Marshal(nil)
	}
	if err != nil {
		return nil, err
	}
	return ret, err
}

func (that *JSONNode) unmarshalMap(data []byte) error {
	tmp := make(map[string]json.RawMessage)
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	for k := range tmp {
		if _, ok := that.m[k]; ok {
			err := json.Unmarshal(tmp[k], that.m[k])
			if err != nil {
				return err
			}
		} else if !that.dontExpand {
			err := json.Unmarshal(tmp[k], that.Map(k))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (that *JSONNode) unmarshalArray(data []byte) error {
	var tmp []json.RawMessage
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	for i := len(tmp) - 1; i >= 0; i-- {
		if !that.dontExpand || i < len(that.a) {
			err := json.Unmarshal(tmp[i], that.At(i))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (that *JSONNode) unmarshalValue(data []byte) error {
	if that.v != nil {
		return json.Unmarshal(data, that.v)
	}
	var tmp interface{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	that.Val(tmp)
	return nil
}

//UnmarshalJSON Make JSONNode a Unmarshaler Interface compatible
func (that *JSONNode) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if that.dontExpand && that.t == TypeUndefined {
		return nil
	}
	if that.t == TypeValue {
		return that.unmarshalValue(data)
	}
	if data[0] == '{' {
		if that.t != TypeMap && that.t != TypeUndefined {
			return ErrorTypeUnmarshaling
		}
		return that.unmarshalMap(data)
	}
	if data[0] == '[' {
		if that.t != TypeArray && that.t != TypeUndefined {
			return ErrorTypeUnmarshaling
		}
		return that.unmarshalArray(data)

	}
	if that.t == TypeUndefined {
		return that.unmarshalValue(data)
	}
	return ErrorTypeUnmarshaling
}
