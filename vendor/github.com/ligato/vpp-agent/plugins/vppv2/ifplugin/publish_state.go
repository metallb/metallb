package ifplugin

import (
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/health/statuscheck"
	"github.com/ligato/cn-infra/health/statuscheck/model/status"

	"github.com/ligato/vpp-agent/api/models/vpp"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
)

// watchStatusEvents watches for resync event of interface state data.
func (p *IfPlugin) watchStatusEvents() {
	defer p.wg.Done()
	p.Log.Debug("Start watching interface state events")

	for {
		select {
		case e := <-p.resyncStatusChan:
			p.onStatusResyncEvent(e)

		case <-p.ctx.Done():
			p.Log.Debug("Stop watching interface state events")
			return
		}
	}
}

// onStatusResyncEvent is triggered during resync of interface state data
func (p *IfPlugin) onStatusResyncEvent(e datasync.ResyncEvent) {
	var wasError error
	for key, vals := range e.GetValues() {
		p.Log.Debugf("trying to delete obsolete status for key %v begin ", key)
		var keys []string
		for {
			x, stop := vals.GetNext()
			if stop {
				break
			}
			keys = append(keys, x.GetKey())
		}
		if len(keys) > 0 {
			err := p.resyncIfStateEvents(keys)
			if err != nil {
				wasError = err
			}
		}
	}
	e.Done(wasError)
}

// resyncIfStateEvents deletes obsolete operation status of network interfaces in DB.
func (p *IfPlugin) resyncIfStateEvents(keys []string) error {
	p.publishLock.Lock()
	defer p.publishLock.Unlock()

	for _, key := range keys {
		ifaceName, isIntfKey := interfaces.ModelInterface.ParseKey(key)
		if !isIntfKey {
			continue
		}

		_, found := p.intfIndex.LookupByName(ifaceName)
		if !found {
			err := p.PublishStatistics.Put(key, nil /*means delete*/)
			if err != nil {
				return err
			}
			p.Log.Debugf("Obsolete interface status for %v deleted", key)
		} else {
			p.Log.WithField("ifaceName", ifaceName).Debug("interface status is needed")
		}
	}

	return nil
}

// publishIfStateEvents goroutine is used to watch interface state notifications
// that are propagated to Messaging topic.
func (p *IfPlugin) publishIfStateEvents() {
	defer p.wg.Done()

	// store last errors to prevent repeating
	var lastPublishErr error
	var lastNotifErr error

	for {
		select {
		case ifState := <-p.ifStateChan:
			p.publishLock.Lock()
			key := interfaces.InterfaceStateKey(ifState.State.Name)

			if p.PublishStatistics != nil {
				err := p.PublishStatistics.Put(key, ifState.State)
				if err != nil {
					if lastPublishErr == nil || lastPublishErr.Error() != err.Error() {
						p.Log.Error(err)
					}
				}
				lastPublishErr = err
			}

			// Marshall data into JSON & send kafka message.
			if p.NotifyStates != nil && ifState.Type == interfaces.InterfaceNotification_UPDOWN {
				err := p.NotifyStates.Put(key, ifState.State)
				if err != nil {
					if lastNotifErr == nil || lastNotifErr.Error() != err.Error() {
						p.Log.Error(err)
					}
				}
				lastNotifErr = err
			}

			// Send interface state data to global agent status
			if p.statusCheckReg {
				p.StatusCheck.ReportStateChangeWithMeta(p.PluginName, statuscheck.OK, nil, &status.InterfaceStats_Interface{
					InternalName: ifState.State.InternalName,
					Index:        ifState.State.IfIndex,
					Status:       ifState.State.AdminStatus.String(),
					MacAddress:   ifState.State.PhysAddress,
				})
			}

			if p.PushNotification != nil &&
				(ifState.Type == interfaces.InterfaceNotification_UPDOWN ||
					ifState.State.OperStatus == interfaces.InterfaceState_DELETED) {
				p.PushNotification(&vpp.Notification{
					Interface: ifState,
				})
			}

			p.publishLock.Unlock()

		case <-p.ctx.Done():
			// Stop watching for state data updates.
			return
		}
	}
}
