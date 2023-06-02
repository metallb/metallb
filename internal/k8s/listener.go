// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"sync"

	"github.com/go-kit/log"
	"go.universe.tf/metallb/api/v1beta1"
	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s/controllers"
	"go.universe.tf/metallb/internal/k8s/epslices"
	v1 "k8s.io/api/core/v1"
)

type Listener struct {
	sync.Mutex
	ServiceChanged func(log.Logger, string, *v1.Service, epslices.EpsOrSlices) controllers.SyncState
	ConfigChanged  func(log.Logger, *config.Config) controllers.SyncState
	PoolChanged    func(log.Logger, *config.Pools) controllers.SyncState
	NodeChanged    func(log.Logger, *v1.Node) controllers.SyncState

	StatusForPool func(string) (v1beta1.IPAddressPoolStatus, error)
}

func (l *Listener) ServiceHandler(logger log.Logger, serviceName string, svc *v1.Service, endpointsOrSlices epslices.EpsOrSlices) controllers.SyncState {
	l.Lock()
	defer l.Unlock()
	return l.ServiceChanged(logger, serviceName, svc, endpointsOrSlices)
}

func (l *Listener) ConfigHandler(logger log.Logger, config *config.Config) controllers.SyncState {
	l.Lock()
	defer l.Unlock()
	return l.ConfigChanged(logger, config)
}

func (l *Listener) NodeHandler(logger log.Logger, node *v1.Node) controllers.SyncState {
	l.Lock()
	defer l.Unlock()
	return l.NodeChanged(logger, node)
}

func (l *Listener) PoolHandler(logger log.Logger, pools *config.Pools) controllers.SyncState {
	l.Lock()
	defer l.Unlock()
	return l.PoolChanged(logger, pools)
}

func (l *Listener) GetStatusForPool(pool string) (v1beta1.IPAddressPoolStatus, error) {
	l.Lock()
	defer l.Unlock()
	return l.StatusForPool(pool)
}
