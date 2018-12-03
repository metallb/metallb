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
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bennyscetbun/jsongo"
)

func getTypeByRef(ctx *context, ref string) *Type {
	for _, typ := range ctx.packageData.Types {
		if ref == toApiType(typ.Name) {
			return &typ
		}
	}
	return nil
}

func getUnionSize(ctx *context, union *Union) (maxSize int) {
	for _, field := range union.Fields {
		if typ := getTypeByRef(ctx, field.Type); typ != nil {
			if size := getSizeOfType(typ); size > maxSize {
				maxSize = size
			}
		}
	}
	return
}

// toApiType returns name that is used as type reference in VPP binary API
func toApiType(name string) string {
	return fmt.Sprintf("vl_api_%s_t", name)
}

// parsePackage parses provided JSON data into objects prepared for code generation
func parsePackage(ctx *context, jsonRoot *jsongo.JSONNode) (*Package, error) {
	logf(" %s contains: %d services, %d messages, %d types, %d enums, %d unions (version: %s)",
		ctx.packageName,
		jsonRoot.Map("services").Len(),
		jsonRoot.Map("messages").Len(),
		jsonRoot.Map("types").Len(),
		jsonRoot.Map("enums").Len(),
		jsonRoot.Map("unions").Len(),
		jsonRoot.Map("vl_api_version").Get(),
	)

	pkg := Package{
		APIVersion: jsonRoot.Map("vl_api_version").Get().(string),
		RefMap:     make(map[string]string),
	}

	// parse enums
	enums := jsonRoot.Map("enums")
	pkg.Enums = make([]Enum, enums.Len())
	for i := 0; i < enums.Len(); i++ {
		enumNode := enums.At(i)

		enum, err := parseEnum(ctx, enumNode)
		if err != nil {
			return nil, err
		}
		pkg.Enums[i] = *enum
		pkg.RefMap[toApiType(enum.Name)] = enum.Name
	}

	// parse types
	types := jsonRoot.Map("types")
	pkg.Types = make([]Type, types.Len())
	for i := 0; i < types.Len(); i++ {
		typNode := types.At(i)

		typ, err := parseType(ctx, typNode)
		if err != nil {
			return nil, err
		}
		pkg.Types[i] = *typ
		pkg.RefMap[toApiType(typ.Name)] = typ.Name
	}

	// parse unions
	unions := jsonRoot.Map("unions")
	pkg.Unions = make([]Union, unions.Len())
	for i := 0; i < unions.Len(); i++ {
		unionNode := unions.At(i)

		union, err := parseUnion(ctx, unionNode)
		if err != nil {
			return nil, err
		}
		pkg.Unions[i] = *union
		pkg.RefMap[toApiType(union.Name)] = union.Name
	}

	// parse messages
	messages := jsonRoot.Map("messages")
	pkg.Messages = make([]Message, messages.Len())
	for i := 0; i < messages.Len(); i++ {
		msgNode := messages.At(i)

		msg, err := parseMessage(ctx, msgNode)
		if err != nil {
			return nil, err
		}
		pkg.Messages[i] = *msg
	}

	// parse services
	services := jsonRoot.Map("services")
	if services.GetType() == jsongo.TypeMap {
		pkg.Services = make([]Service, services.Len())
		for i, key := range services.GetKeys() {
			svcNode := services.At(key)

			svc, err := parseService(ctx, key.(string), svcNode)
			if err != nil {
				return nil, err
			}
			pkg.Services[i] = *svc
		}

		// sort services
		sort.Slice(pkg.Services, func(i, j int) bool {
			// dumps first
			if pkg.Services[i].Stream != pkg.Services[j].Stream {
				return pkg.Services[i].Stream
			}
			return pkg.Services[i].RequestType < pkg.Services[j].RequestType
		})
	}

	printPackage(&pkg)

	return &pkg, nil
}

// printPackage prints all loaded objects for package
func printPackage(pkg *Package) {
	if len(pkg.Enums) > 0 {
		logf("loaded %d enums:", len(pkg.Enums))
		for k, enum := range pkg.Enums {
			logf(" - enum #%d\t%+v", k, enum)
		}
	}
	if len(pkg.Unions) > 0 {
		logf("loaded %d unions:", len(pkg.Unions))
		for k, union := range pkg.Unions {
			logf(" - union #%d\t%+v", k, union)
		}
	}
	if len(pkg.Types) > 0 {
		logf("loaded %d types:", len(pkg.Types))
		for _, typ := range pkg.Types {
			logf(" - type: %q (%d fields)", typ.Name, len(typ.Fields))
		}
	}
	if len(pkg.Messages) > 0 {
		logf("loaded %d messages:", len(pkg.Messages))
		for _, msg := range pkg.Messages {
			logf(" - message: %q (%d fields)", msg.Name, len(msg.Fields))
		}
	}
	if len(pkg.Services) > 0 {
		logf("loaded %d services:", len(pkg.Services))
		for _, svc := range pkg.Services {
			var info string
			if svc.Stream {
				info = "(STREAM)"
			} else if len(svc.Events) > 0 {
				info = fmt.Sprintf("(EVENTS: %v)", svc.Events)
			}
			logf(" - service: %q -> %q %s", svc.RequestType, svc.ReplyType, info)
		}
	}
}

// parseEnum parses VPP binary API enum object from JSON node
func parseEnum(ctx *context, enumNode *jsongo.JSONNode) (*Enum, error) {
	if enumNode.Len() == 0 || enumNode.At(0).GetType() != jsongo.TypeValue {
		return nil, errors.New("invalid JSON for enum specified")
	}

	enumName, ok := enumNode.At(0).Get().(string)
	if !ok {
		return nil, fmt.Errorf("enum name is %T, not a string", enumNode.At(0).Get())
	}
	enumType, ok := enumNode.At(enumNode.Len() - 1).At("enumtype").Get().(string)
	if !ok {
		return nil, fmt.Errorf("enum type invalid or missing")
	}

	enum := Enum{
		Name: enumName,
		Type: enumType,
	}

	// loop through enum entries, skip first (name) and last (enumtype)
	for j := 1; j < enumNode.Len()-1; j++ {
		if enumNode.At(j).GetType() == jsongo.TypeArray {
			entry := enumNode.At(j)

			if entry.Len() < 2 || entry.At(0).GetType() != jsongo.TypeValue || entry.At(1).GetType() != jsongo.TypeValue {
				return nil, errors.New("invalid JSON for enum entry specified")
			}

			entryName, ok := entry.At(0).Get().(string)
			if !ok {
				return nil, fmt.Errorf("enum entry name is %T, not a string", entry.At(0).Get())
			}
			entryVal := entry.At(1).Get()

			enum.Entries = append(enum.Entries, EnumEntry{
				Name:  entryName,
				Value: entryVal,
			})
		}
	}

	return &enum, nil
}

// parseUnion parses VPP binary API union object from JSON node
func parseUnion(ctx *context, unionNode *jsongo.JSONNode) (*Union, error) {
	if unionNode.Len() == 0 || unionNode.At(0).GetType() != jsongo.TypeValue {
		return nil, errors.New("invalid JSON for union specified")
	}

	unionName, ok := unionNode.At(0).Get().(string)
	if !ok {
		return nil, fmt.Errorf("union name is %T, not a string", unionNode.At(0).Get())
	}
	unionCRC, ok := unionNode.At(unionNode.Len() - 1).At("crc").Get().(string)
	if !ok {
		return nil, fmt.Errorf("union crc invalid or missing")
	}

	union := Union{
		Name: unionName,
		CRC:  unionCRC,
	}

	// loop through union fields, skip first (name) and last (crc)
	for j := 1; j < unionNode.Len()-1; j++ {
		if unionNode.At(j).GetType() == jsongo.TypeArray {
			fieldNode := unionNode.At(j)

			field, err := parseField(ctx, fieldNode)
			if err != nil {
				return nil, err
			}

			union.Fields = append(union.Fields, *field)
		}
	}

	return &union, nil
}

// parseType parses VPP binary API type object from JSON node
func parseType(ctx *context, typeNode *jsongo.JSONNode) (*Type, error) {
	if typeNode.Len() == 0 || typeNode.At(0).GetType() != jsongo.TypeValue {
		return nil, errors.New("invalid JSON for type specified")
	}

	typeName, ok := typeNode.At(0).Get().(string)
	if !ok {
		return nil, fmt.Errorf("type name is %T, not a string", typeNode.At(0).Get())
	}
	typeCRC, ok := typeNode.At(typeNode.Len() - 1).At("crc").Get().(string)
	if !ok {
		return nil, fmt.Errorf("type crc invalid or missing")
	}

	typ := Type{
		Name: typeName,
		CRC:  typeCRC,
	}

	// loop through type fields, skip first (name) and last (crc)
	for j := 1; j < typeNode.Len()-1; j++ {
		if typeNode.At(j).GetType() == jsongo.TypeArray {
			fieldNode := typeNode.At(j)

			field, err := parseField(ctx, fieldNode)
			if err != nil {
				return nil, err
			}

			typ.Fields = append(typ.Fields, *field)
		}
	}

	return &typ, nil
}

// parseMessage parses VPP binary API message object from JSON node
func parseMessage(ctx *context, msgNode *jsongo.JSONNode) (*Message, error) {
	if msgNode.Len() == 0 || msgNode.At(0).GetType() != jsongo.TypeValue {
		return nil, errors.New("invalid JSON for message specified")
	}

	msgName, ok := msgNode.At(0).Get().(string)
	if !ok {
		return nil, fmt.Errorf("message name is %T, not a string", msgNode.At(0).Get())
	}
	msgCRC, ok := msgNode.At(msgNode.Len() - 1).At("crc").Get().(string)
	if !ok {

		return nil, fmt.Errorf("message crc invalid or missing")
	}

	msg := Message{
		Name: msgName,
		CRC:  msgCRC,
	}

	// loop through message fields, skip first (name) and last (crc)
	for j := 1; j < msgNode.Len()-1; j++ {
		if msgNode.At(j).GetType() == jsongo.TypeArray {
			fieldNode := msgNode.At(j)

			field, err := parseField(ctx, fieldNode)
			if err != nil {
				return nil, err
			}

			msg.Fields = append(msg.Fields, *field)
		}
	}

	return &msg, nil
}

// parseField parses VPP binary API object field from JSON node
func parseField(ctx *context, field *jsongo.JSONNode) (*Field, error) {
	if field.Len() < 2 || field.At(0).GetType() != jsongo.TypeValue || field.At(1).GetType() != jsongo.TypeValue {
		return nil, errors.New("invalid JSON for field specified")
	}

	fieldType, ok := field.At(0).Get().(string)
	if !ok {
		return nil, fmt.Errorf("field type is %T, not a string", field.At(0).Get())
	}
	fieldName, ok := field.At(1).Get().(string)
	if !ok {
		return nil, fmt.Errorf("field name is %T, not a string", field.At(1).Get())
	}
	var fieldLength float64
	if field.Len() >= 3 {
		fieldLength, ok = field.At(2).Get().(float64)
		if !ok {
			return nil, fmt.Errorf("field length is %T, not an int", field.At(2).Get())
		}
	}
	var fieldLengthFrom string
	if field.Len() >= 4 {
		fieldLengthFrom, ok = field.At(3).Get().(string)
		if !ok {
			return nil, fmt.Errorf("field length from is %T, not a string", field.At(3).Get())
		}
	}

	return &Field{
		Name:     fieldName,
		Type:     fieldType,
		Length:   int(fieldLength),
		SizeFrom: fieldLengthFrom,
	}, nil
}

// parseService parses VPP binary API service object from JSON node
func parseService(ctx *context, svcName string, svcNode *jsongo.JSONNode) (*Service, error) {
	if svcNode.Len() == 0 || svcNode.At("reply").GetType() != jsongo.TypeValue {
		return nil, errors.New("invalid JSON for service specified")
	}

	svc := Service{
		Name:        ctx.moduleName + "." + svcName,
		RequestType: svcName,
	}

	if replyNode := svcNode.At("reply"); replyNode.GetType() == jsongo.TypeValue {
		reply, ok := replyNode.Get().(string)
		if !ok {
			return nil, fmt.Errorf("service reply is %T, not a string", replyNode.Get())
		}
		if reply != "null" {
			svc.ReplyType = reply
		}
	}

	// stream service (dumps)
	if streamNode := svcNode.At("stream"); streamNode.GetType() == jsongo.TypeValue {
		var ok bool
		svc.Stream, ok = streamNode.Get().(bool)
		if !ok {
			return nil, fmt.Errorf("service stream is %T, not a string", streamNode.Get())
		}
	}

	// events service (event subscription)
	if eventsNode := svcNode.At("events"); eventsNode.GetType() == jsongo.TypeArray {
		for j := 0; j < eventsNode.Len(); j++ {
			event := eventsNode.At(j).Get().(string)
			svc.Events = append(svc.Events, event)
		}
	}

	// validate service
	if svc.IsEventService() {
		if !strings.HasPrefix(svc.RequestType, "want_") {
			log.Warnf("Unusual EVENTS SERVICE: %+v\n"+
				"- events service %q does not have 'want_' prefix in request.",
				svc, svc.Name)
		}
	} else if svc.IsDumpService() {
		if !strings.HasSuffix(svc.RequestType, "_dump") ||
			!strings.HasSuffix(svc.ReplyType, "_details") {
			log.Warnf("Unusual STREAM SERVICE: %+v\n"+
				"- stream service %q does not have '_dump' suffix in request or reply does not have '_details' suffix.",
				svc, svc.Name)
		}
	} else if svc.IsRequestService() {
		if !strings.HasSuffix(svc.ReplyType, "_reply") {
			log.Warnf("Unusual REQUEST SERVICE: %+v\n"+
				"- service %q does not have '_reply' suffix in reply.",
				svc, svc.Name)
		}
	}

	return &svc, nil
}

// convertToGoType translates the VPP binary API type into Go type
func convertToGoType(ctx *context, binapiType string) (typ string) {
	if t, ok := binapiTypes[binapiType]; ok {
		// basic types
		typ = t
	} else if r, ok := ctx.packageData.RefMap[binapiType]; ok {
		// specific types (enums/types/unions)
		typ = camelCaseName(r)
	} else {
		// fallback type
		log.Warnf("found unknown VPP binary API type %q, using byte", binapiType)
		typ = "byte"
	}
	return typ
}
