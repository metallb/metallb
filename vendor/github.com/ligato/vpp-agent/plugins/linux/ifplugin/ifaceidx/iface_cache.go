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
	"strings"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/vpp-agent/idxvpp/cacheutil"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	linux_ifaces "github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
)

// Cache the VETH interfaces of a particular agent by watching transport.
// If change appears, it is registered in idx map.
func Cache(watcher datasync.KeyValProtoWatcher) LinuxIfIndex {
	resyncName := fmt.Sprintf("linux-iface-cache-%s", watcher)
	linuxIfIdx := NewLinuxIfIndex(nametoidx.NewNameToIdx(logrus.DefaultLogger(), resyncName, IndexMetadata))

	helper := cacheutil.CacheHelper{
		Prefix:        linux_ifaces.InterfaceKeyPrefix(),
		IDX:           linuxIfIdx.GetMapping(),
		DataPrototype: &linux_ifaces.LinuxInterfaces_Interface{Name: "linux_iface"},
		ParseName:     ParseNameFromKey,
	}

	go helper.DoWatching(resyncName, watcher)

	return linuxIfIdx
}

// ParseNameFromKey returns suffix of the key (name).
func ParseNameFromKey(key string) (name string, err error) {
	lastSlashPos := strings.LastIndex(key, "/")
	if lastSlashPos > 0 && lastSlashPos < len(key)-1 {
		return key[lastSlashPos+1:], nil
	}

	return key, fmt.Errorf("incorrect format of the key %s", key)
}
