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

package ifaceidx

import (
	"fmt"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/cacheutil"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
)

// Cache the network interfaces of a particular agent by watching (ETCD or different transport).
// Beware: the indices in cache do not correspond with the real indices.
func Cache(watcher datasync.KeyValProtoWatcher) SwIfIndex {
	resyncName := fmt.Sprintf("iface-cache-%s", watcher)
	swIdx := NewSwIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), resyncName, IndexMetadata))

	helper := cacheutil.CacheHelper{
		Prefix:        interfaces.Prefix,
		IDX:           swIdx.GetMapping(),
		DataPrototype: &interfaces.Interfaces_Interface{Name: "aaa"},
		ParseName:     interfaces.ParseNameFromKey}

	go helper.DoWatching(resyncName, watcher)

	return swIdx
}
