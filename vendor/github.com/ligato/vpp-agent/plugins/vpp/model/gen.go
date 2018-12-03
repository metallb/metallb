//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

//go:generate protoc --proto_path=acl --gogo_out=acl acl/acl.proto
//go:generate protoc --proto_path=bfd --gogo_out=bfd bfd/bfd.proto
//go:generate protoc --proto_path=interfaces --gogo_out=interfaces interfaces/interfaces.proto
//go:generate protoc --proto_path=ipsec --gogo_out=ipsec ipsec/ipsec.proto
//go:generate protoc --proto_path=l2 --gogo_out=l2 l2/l2.proto
//go:generate protoc --proto_path=l3 --gogo_out=l3 l3/l3.proto
//go:generate protoc --proto_path=l4 --gogo_out=l4 l4/l4.proto
//go:generate protoc --proto_path=nat --gogo_out=nat nat.proto
//go:generate protoc --proto_path=rpc --proto_path=$GOPATH/src --gogo_out=plugins=grpc:rpc rpc/rpc.proto
//go:generate protoc --proto_path=srv6 --gogo_out=srv6 srv6/srv6.proto
//go:generate protoc --proto_path=stn --gogo_out=stn stn/stn.proto

package model
