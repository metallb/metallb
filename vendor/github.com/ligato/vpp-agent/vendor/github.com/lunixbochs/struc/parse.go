package struc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// struc:"int32,big,sizeof=Data"

type strucTag struct {
	Type   string
	Order  binary.ByteOrder
	Sizeof string
	Skip   bool
}

func parseStrucTag(tag reflect.StructTag) *strucTag {
	t := &strucTag{
		Order: binary.BigEndian,
	}
	tagStr := tag.Get("struc")
	if tagStr == "" {
		// someone's going to typo this (I already did once)
		// sorry if you made a module actually using this tag
		// and you're mad at me now
		tagStr = tag.Get("struct")
	}
	for _, s := range strings.Split(tagStr, ",") {
		if strings.HasPrefix(s, "sizeof=") {
			tmp := strings.SplitN(s, "=", 2)
			t.Sizeof = tmp[1]
		} else if s == "big" {
			t.Order = binary.BigEndian
		} else if s == "little" {
			t.Order = binary.LittleEndian
		} else if s == "skip" {
			t.Skip = true
		} else {
			t.Type = s
		}
	}
	return t
}

var typeLenRe = regexp.MustCompile(`^\[(\d*)\]`)

func parseField(f reflect.StructField) (fd *Field, tag *strucTag, err error) {
	tag = parseStrucTag(f.Tag)
	var ok bool
	fd = &Field{
		Name:  f.Name,
		Len:   1,
		Order: tag.Order,
		Slice: false,
		kind:  f.Type.Kind(),
	}
	switch fd.kind {
	case reflect.Array:
		fd.Slice = true
		fd.Array = true
		fd.Len = f.Type.Len()
		fd.kind = f.Type.Elem().Kind()
	case reflect.Slice:
		fd.Slice = true
		fd.Len = -1
		fd.kind = f.Type.Elem().Kind()
	case reflect.Ptr:
		fd.Ptr = true
		fd.kind = f.Type.Elem().Kind()
	}
	// check for custom types
	tmp := reflect.New(f.Type)
	if _, ok := tmp.Interface().(Custom); ok {
		fd.Type = CustomType
		return
	}
	var defTypeOk bool
	fd.defType, defTypeOk = reflectTypeMap[fd.kind]
	// find a type in the struct tag
	pureType := typeLenRe.ReplaceAllLiteralString(tag.Type, "")
	if fd.Type, ok = typeLookup[pureType]; ok {
		fd.Len = 1
		match := typeLenRe.FindAllStringSubmatch(tag.Type, -1)
		if len(match) > 0 && len(match[0]) > 1 {
			fd.Slice = true
			first := match[0][1]
			// Field.Len = -1 indicates a []slice
			if first == "" {
				fd.Len = -1
			} else {
				fd.Len, err = strconv.Atoi(first)
			}
		}
		return
	}
	// the user didn't specify a type
	switch f.Type {
	case reflect.TypeOf(Size_t(0)):
		fd.Type = SizeType
	case reflect.TypeOf(Off_t(0)):
		fd.Type = OffType
	default:
		if defTypeOk {
			fd.Type = fd.defType
		} else {
			err = errors.New("struc: Could not find field type.")
		}
	}
	return
}

func parseFieldsLocked(v reflect.Value) (Fields, error) {
	// we need to repeat this logic because parseFields() below can't be recursively called due to locking
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	if v.NumField() < 1 {
		return nil, errors.New("struc: Struct has no fields.")
	}
	sizeofMap := make(map[string][]int)
	fields := make(Fields, v.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		f, tag, err := parseField(field)
		if tag.Skip {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !v.Field(i).CanSet() {
			continue
		}
		f.Index = i
		if tag.Sizeof != "" {
			target, ok := t.FieldByName(tag.Sizeof)
			if !ok {
				return nil, fmt.Errorf("struc: `sizeof=%s` field does not exist", tag.Sizeof)
			}
			f.Sizeof = target.Index
			sizeofMap[tag.Sizeof] = field.Index
		}
		if sizefrom, ok := sizeofMap[field.Name]; ok {
			f.Sizefrom = sizefrom
		}
		if f.Len == -1 && f.Sizefrom == nil {
			return nil, fmt.Errorf("struc: field `%s` is a slice with no length or sizeof field", field.Name)
		}
		// recurse into nested structs
		// TODO: handle loops (probably by indirecting the []Field and putting pointer in cache)
		if f.Type == Struct {
			typ := field.Type
			if f.Ptr {
				typ = typ.Elem()
			}
			if f.Slice {
				typ = typ.Elem()
			}
			f.Fields, err = parseFieldsLocked(reflect.New(typ))
			if err != nil {
				return nil, err
			}
		}
		fields[i] = f
	}
	return fields, nil
}

var fieldCache = make(map[reflect.Type]Fields)
var fieldCacheLock sync.RWMutex
var parseLock sync.Mutex

func fieldCacheLookup(t reflect.Type) Fields {
	fieldCacheLock.RLock()
	defer fieldCacheLock.RUnlock()
	if cached, ok := fieldCache[t]; ok {
		return cached
	}
	return nil
}

func parseFields(v reflect.Value) (Fields, error) {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	// fast path: hopefully the field parsing is already cached
	if cached := fieldCacheLookup(t); cached != nil {
		return cached, nil
	}

	// hold a global lock so multiple goroutines can't parse (the same) fields at once
	parseLock.Lock()
	defer parseLock.Unlock()

	// check cache a second time, in case parseLock was just released by
	// another thread who filled the cache for us
	if cached := fieldCacheLookup(t); cached != nil {
		return cached, nil
	}

	// no luck, time to parse and fill the cache ourselves
	fields, err := parseFieldsLocked(v)
	if err != nil {
		return nil, err
	}
	fieldCacheLock.Lock()
	fieldCache[t] = fields
	fieldCacheLock.Unlock()
	return fields, nil
}
