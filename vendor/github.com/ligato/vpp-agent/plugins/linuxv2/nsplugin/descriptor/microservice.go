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

package descriptor

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/fsouza/go-dockerclient"
	prototypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/servicelabel"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"

	nsmodel "github.com/ligato/vpp-agent/api/models/linux/namespace"
)

const (
	// MicroserviceDescriptorName is the name of the descriptor for microservices.
	MicroserviceDescriptorName = "microservice"

	// how often in seconds to refresh the microservice state data
	dockerRefreshPeriod = 3 * time.Second
	dockerRetryPeriod   = 5 * time.Second
)

// MicroserviceDescriptor watches Docker and notifies KVScheduler about newly
// started and stopped microservices.
type MicroserviceDescriptor struct {
	// input arguments
	log         logging.Logger
	kvscheduler kvs.KVScheduler

	// map microservice label -> time of the last creation
	createTime map[string]time.Time

	// lock used to serialize access to microservice state data
	msStateLock sync.Mutex

	// conditional variable to check if microservice state data are in-sync
	// with the docker
	msStateInSync     bool
	msStateInSyncCond *sync.Cond

	// docker client - used to convert microservice label into the PID and
	// ID of the container
	dockerClient *docker.Client
	// microservice label -> microservice state data
	microServiceByLabel map[string]*Microservice
	// microservice container ID -> microservice state data
	microServiceByID map[string]*Microservice

	// go routine management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Microservice is used to store PID and ID of the container running a given
// microservice.
type Microservice struct {
	Label string
	PID   int
	ID    string
}

// microserviceCtx contains all data required to handle microservice changes.
type microserviceCtx struct {
	created       []string
	since         string
	lastInspected int64
}

// NewMicroserviceDescriptor creates a new instance of the descriptor for microservices.
func NewMicroserviceDescriptor(kvscheduler kvs.KVScheduler, log logging.PluginLogger) (*MicroserviceDescriptor, error) {
	var err error

	descriptor := &MicroserviceDescriptor{
		log:                 log.NewLogger("ms-descriptor"),
		kvscheduler:         kvscheduler,
		createTime:          make(map[string]time.Time),
		microServiceByLabel: make(map[string]*Microservice),
		microServiceByID:    make(map[string]*Microservice),
	}
	descriptor.msStateInSyncCond = sync.NewCond(&descriptor.msStateLock)
	descriptor.ctx, descriptor.cancel = context.WithCancel(context.Background())

	// Docker client
	descriptor.dockerClient, err = docker.NewClientFromEnv()
	if err != nil {
		return nil, errors.Errorf("failed to get docker client instance from the environment variables: %v", err)
	}
	log.Debugf("Using docker client endpoint: %+v", descriptor.dockerClient.Endpoint())

	return descriptor, nil
}

// GetDescriptor returns descriptor suitable for registration with the KVScheduler.
func (d *MicroserviceDescriptor) GetDescriptor() *kvs.KVDescriptor {
	return &kvs.KVDescriptor{
		Name:        MicroserviceDescriptorName,
		KeySelector: d.IsMicroserviceKey,
		Dump:        d.Dump,
	}
}

// IsMicroserviceKey returns true for key identifying microservices.
func (d *MicroserviceDescriptor) IsMicroserviceKey(key string) bool {
	return strings.HasPrefix(key, nsmodel.MicroserviceKeyPrefix)
}

// Dump returns key with empty value for every currently existing microservice.
func (d *MicroserviceDescriptor) Dump(correlate []kvs.KVWithMetadata) (dump []kvs.KVWithMetadata, err error) {
	// wait until microservice state data are in-sync with the docker
	d.msStateLock.Lock()
	if !d.msStateInSync {
		d.msStateInSyncCond.Wait()
	}
	defer d.msStateLock.Unlock()

	for msLabel := range d.microServiceByLabel {
		dump = append(dump, kvs.KVWithMetadata{
			Key:    nsmodel.MicroserviceKey(msLabel),
			Value:  &prototypes.Empty{},
			Origin: kvs.FromSB,
		})
	}

	return dump, nil
}

// StartTracker starts microservice tracker,
func (d *MicroserviceDescriptor) StartTracker() {
	go d.trackMicroservices(d.ctx)
}

// StopTracker stops microservice tracker,
func (d *MicroserviceDescriptor) StopTracker() {
	d.cancel()
	d.wg.Wait()
}

// GetMicroserviceStateData returns state data for the given microservice.
func (d *MicroserviceDescriptor) GetMicroserviceStateData(msLabel string) (ms *Microservice, found bool) {
	d.msStateLock.Lock()
	if !d.msStateInSync {
		d.msStateInSyncCond.Wait()
	}
	defer d.msStateLock.Unlock()

	ms, found = d.microServiceByLabel[msLabel]
	return ms, found
}

// handleMicroservices handles microservice changes.
func (d *MicroserviceDescriptor) handleMicroservices(ctx *microserviceCtx) {
	var err error
	var newest int64
	var containers []docker.APIContainers
	var nextCreated []string

	// First check if any microservice has terminated.
	d.msStateLock.Lock()
	for container := range d.microServiceByID {
		details, err := d.dockerClient.InspectContainer(container)
		if err != nil || !details.State.Running {
			d.processTerminatedMicroservice(container)
		}
	}
	d.msStateLock.Unlock()

	// Now check if previously created containers have transitioned to the state "running".
	for _, container := range ctx.created {
		details, err := d.dockerClient.InspectContainer(container)
		if err == nil {
			if details.State.Running {
				d.detectMicroservice(details)
			} else if details.State.Status == "created" {
				nextCreated = append(nextCreated, container)
			}
		} else {
			d.log.Debugf("Inspect container ID %v failed: %v", container, err)
		}
	}
	ctx.created = nextCreated

	// Inspect newly created containers
	listOpts := docker.ListContainersOptions{
		All:     true,
		Filters: map[string][]string{},
	}
	// List containers and filter all older than 'since' ID
	if ctx.since != "" {
		listOpts.Filters["since"] = []string{ctx.since}
	}
	containers, err = d.dockerClient.ListContainers(listOpts)
	if err != nil {
		// If 'since' container was not found, list all containers (404 is required to support older docker version)
		if dockerErr, ok := err.(*docker.Error); ok && (dockerErr.Status == 500 || dockerErr.Status == 404) {
			// Reset filter and list containers again
			d.log.Debugf("clearing 'since' %s", ctx.since)
			ctx.since = ""
			delete(listOpts.Filters, "since")
			containers, err = d.dockerClient.ListContainers(listOpts)
		}
		if err != nil {
			// If there is other error, return it
			d.log.Errorf("Error listing docker containers: %v", err)
			return
		}
	}

	for _, container := range containers {
		d.log.Debugf("processing new container %v with state %v", container.ID, container.State)
		if container.State == "running" && container.Created > ctx.lastInspected {
			// Inspect the container to get the list of defined environment variables.
			details, err := d.dockerClient.InspectContainer(container.ID)
			if err != nil {
				d.log.Debugf("Inspect container %v failed: %v", container.ID, err)
				continue
			}
			d.detectMicroservice(details)
		}
		if container.State == "created" {
			ctx.created = append(ctx.created, container.ID)
		}
		if container.Created > newest {
			newest = container.Created
			ctx.since = container.ID
		}
	}

	if newest > ctx.lastInspected {
		ctx.lastInspected = newest
	}
}

// detectMicroservice inspects container to see if it is a microservice.
// If microservice is detected, processNewMicroservice() is called to process it.
func (d *MicroserviceDescriptor) detectMicroservice(container *docker.Container) {
	// Search for the microservice label.
	var label string
	for _, env := range container.Config.Env {
		if strings.HasPrefix(env, servicelabel.MicroserviceLabelEnvVar+"=") {
			label = env[len(servicelabel.MicroserviceLabelEnvVar)+1:]
			if label != "" {
				d.log.Debugf("detected container as microservice: Name=%v ID=%v Created=%v State.StartedAt=%v", container.Name, container.ID, container.Created, container.State.StartedAt)
				last := d.createTime[label]
				if last.After(container.Created) {
					d.log.Debugf("ignoring older container created at %v as microservice: %+v", last, container)
					continue
				}
				d.createTime[label] = container.Created
				d.processNewMicroservice(label, container.ID, container.State.Pid)
			}
		}
	}
}

// processNewMicroservice is triggered every time a new microservice gets freshly started. All pending interfaces are moved
// to its namespace.
func (d *MicroserviceDescriptor) processNewMicroservice(microserviceLabel string, id string, pid int) {
	d.msStateLock.Lock()
	defer d.msStateLock.Unlock()

	ms, restarted := d.microServiceByLabel[microserviceLabel]
	if restarted {
		d.processTerminatedMicroservice(ms.ID)
		d.log.WithFields(logging.Fields{"label": microserviceLabel, "new-pid": pid, "new-id": id}).
			Warn("Microservice has been restarted")
	} else {
		d.log.WithFields(logging.Fields{"label": microserviceLabel, "pid": pid, "id": id}).
			Debug("Discovered new microservice")
	}

	ms = &Microservice{Label: microserviceLabel, PID: pid, ID: id}
	d.microServiceByLabel[microserviceLabel] = ms
	d.microServiceByID[id] = ms

	// Notify scheduler about new microservice
	if d.msStateInSync {
		d.kvscheduler.PushSBNotification(
			nsmodel.MicroserviceKey(ms.Label),
			&prototypes.Empty{},
			nil)
	}
}

// processTerminatedMicroservice is triggered every time a known microservice
// has terminated. All associated interfaces become obsolete and are thus removed.
func (d *MicroserviceDescriptor) processTerminatedMicroservice(id string) {
	ms, exists := d.microServiceByID[id]
	if !exists {
		d.log.WithFields(logging.Fields{"id": id}).
			Warn("Detected removal of an unknown microservice")
		return
	}
	d.log.WithFields(logging.Fields{"label": ms.Label, "pid": ms.PID, "id": ms.ID}).
		Debug("Microservice has terminated")

	delete(d.microServiceByLabel, ms.Label)
	delete(d.microServiceByID, ms.ID)

	// Notify scheduler about terminated microservice
	if d.msStateInSync {
		d.kvscheduler.PushSBNotification(
			nsmodel.MicroserviceKey(ms.Label),
			nil,
			nil)
	}
}

// trackMicroservices is running in the background and maintains a map of microservice labels to container info.
func (d *MicroserviceDescriptor) trackMicroservices(ctx context.Context) {
	d.wg.Add(1)
	defer func() {
		d.wg.Done()
		d.log.Debugf("Microservice tracking ended")
	}()

	msCtx := &microserviceCtx{}

	var clientOk bool

	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			if err := d.dockerClient.Ping(); err != nil {
				if clientOk {
					d.log.Errorf("Docker ping check failed: %v", err)
				}
				clientOk = false

				// Sleep before another retry.
				timer.Reset(dockerRetryPeriod)
				break
			}

			if !clientOk {
				d.log.Infof("Docker ping check OK")
				/*if info, err := d.dockerClient.Info(); err != nil {
					d.log.Errorf("Retrieving docker info failed: %v", err)
					timer.Reset(dockerRetryPeriod)
					continue
				} else {
					d.log.Infof("Docker connection established: server version: %v (%v %v %v)",
						info.ServerVersion, info.OperatingSystem, info.Architecture, info.KernelVersion)
				}*/
			}
			clientOk = true

			d.handleMicroservices(msCtx)

			// Sleep before another refresh.
			timer.Reset(dockerRefreshPeriod)
		case <-d.ctx.Done():
			return
		}

		// mark state data as in-sync - if connection to docker is failing,
		// empty set of microservices is considered
		d.msStateLock.Lock()
		d.msStateInSync = true
		d.msStateLock.Unlock()
		d.msStateInSyncCond.Broadcast()
	}
}
