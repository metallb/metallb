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

package syncbase

import "github.com/ligato/cn-infra/logging/logrus"

// NewDoneChannel creates a new instance of DoneChannel.
func NewDoneChannel(doneChan chan error) *DoneChannel {
	return &DoneChannel{doneChan}
}

// DoneChannel is a small reusable part that is embedded to other events using composition.
// It implements datasync.CallbackResult.
type DoneChannel struct {
	DoneChan chan error
}

// Done propagates error to the channel.
func (ev *DoneChannel) Done(err error) {
	if ev.DoneChan != nil {
		select {
		case ev.DoneChan <- err:
			// sent successfully
		default:
			logrus.DefaultLogger().Debug("Nobody is listening anymore")
		}
	} else if err != nil {
		logrus.DefaultLogger().Error(err)
	}
}

// DoneCallback is a small reusable part that is embedded to other events using composition.
// It implements datasync.CallbackResult.
type DoneCallback struct {
	Callback func(error)
}

// Done propagates error to the callback.
func (ev *DoneCallback) Done(err error) {
	if ev.Callback != nil {
		ev.Callback(err)
	} else if err != nil {
		logrus.DefaultLogger().Error(err)
	}
}

// AggregateDone can be reused to avoid repetitive code that triggers a slice of events and waits until it is finished.
func AggregateDone(events []func(chan error), done chan error) {
	partialDone := make(chan error, 5)

	go collectDoneEvents(partialDone, done, len(events))

	for _, event := range events {
		event(partialDone) // fire event
	}
}

func collectDoneEvents(partialDone, done chan error, count int) {
	var lastErr error

	if count > 0 {
		for i := 0; i < count; i++ {
			if err := <-partialDone; err != nil {
				lastErr = err
			}
		}
		logrus.DefaultLogger().Debug("TX Done - all events callbacks received")
	}

	done <- lastErr
}
