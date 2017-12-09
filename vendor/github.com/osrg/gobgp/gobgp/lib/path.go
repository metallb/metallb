// Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

// typedef struct {
//     char *value;
//     int len;
// } buf;
//
// typedef struct path_t {
//     buf   nlri;
//     buf** path_attributes;
//     int   path_attributes_len;
//     int   path_attributes_cap;
// } path;
// extern path* new_path();
// extern void free_path(path*);
// extern int append_path_attribute(path*, int, char*);
// extern buf* get_path_attribute(path*, int);
import "C"

import (
	"encoding/json"
	"strings"

	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/gobgp/cmd"
	"github.com/osrg/gobgp/packet/bgp"
)

//export get_route_family
func get_route_family(input *C.char) C.int {
	rf, err := bgp.GetRouteFamily(C.GoString(input))
	if err != nil {
		return C.int(-1)
	}
	return C.int(rf)
}

//export serialize_path
func serialize_path(rf C.int, input *C.char) *C.path {
	args := strings.Split(C.GoString(input), " ")
	pp, err := cmd.ParsePath(bgp.RouteFamily(rf), args)
	if err != nil {
		return nil
	}
	path := C.new_path()
	p := api.ToPathApi(pp)
	if len(p.Nlri) > 0 {
		path.nlri.len = C.int(len(p.Nlri))
		path.nlri.value = C.CString(string(p.Nlri))
	}
	for _, attr := range p.Pattrs {
		C.append_path_attribute(path, C.int(len(attr)), C.CString(string(attr)))
	}
	return path
}

//export decode_path
func decode_path(p *C.path) *C.char {
	var buf []byte
	var nlri bgp.AddrPrefixInterface
	if p.nlri.len > 0 {
		buf = []byte(C.GoStringN(p.nlri.value, p.nlri.len))
		nlri = &bgp.IPAddrPrefix{}
		err := nlri.DecodeFromBytes(buf)
		if err != nil {
			return nil
		}
	}
	pattrs := make([]bgp.PathAttributeInterface, 0, int(p.path_attributes_len))
	for i := 0; i < int(p.path_attributes_len); i++ {
		b := C.get_path_attribute(p, C.int(i))
		buf = []byte(C.GoStringN(b.value, b.len))
		pattr, err := bgp.GetPathAttribute(buf)
		if err != nil {
			return nil
		}

		err = pattr.DecodeFromBytes(buf)
		if err != nil {
			return nil
		}

		switch pattr.GetType() {
		case bgp.BGP_ATTR_TYPE_MP_REACH_NLRI:
			mpreach := pattr.(*bgp.PathAttributeMpReachNLRI)
			if len(mpreach.Value) != 1 {
				return nil
			}
			nlri = mpreach.Value[0]
		}

		pattrs = append(pattrs, pattr)
	}
	j, _ := json.Marshal(struct {
		Nlri      bgp.AddrPrefixInterface      `json:"nlri"`
		PathAttrs []bgp.PathAttributeInterface `json:"attrs"`
	}{
		Nlri:      nlri,
		PathAttrs: pattrs,
	})
	return C.CString(string(j))
}

//export decode_capabilities
func decode_capabilities(p *C.buf) *C.char {
	buf := []byte(C.GoStringN(p.value, p.len))
	c, err := bgp.DecodeCapability(buf)
	if err != nil {
		return nil
	}
	j, _ := json.Marshal(c)
	return C.CString(string(j))

}

func main() {
	// We need the main function to make possible
	// CGO compiler to compile the package as C shared library
}
