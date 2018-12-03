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

package govppmux

import (
	"time"

	"github.com/ligato/cn-infra/logging/measure"

	govppapi "git.fd.io/govpp.git/api"
	"git.fd.io/govpp.git/core"
	"github.com/ligato/cn-infra/logging/logrus"
)

// goVppChan implements govpp channel interface. Instance is returned by NewAPIChannel() or NewAPIChannelBuffered(),
// and contains *govpp.channel dynamic type (vppChan field). Implemented methods allow custom handling of low-level
// govpp.
type goVppChan struct {
	govppapi.Channel
	// Retry data
	retry retryConfig
	// tracer used to measure binary api call duration
	tracer measure.Tracer
}

// helper struct holding info about retry configuration
type retryConfig struct {
	attempts int
	timeout  time.Duration
}

// govppRequestCtx is custom govpp RequestCtx.
type govppRequestCtx struct {
	// Original request context
	requestCtx govppapi.RequestCtx
	// Function allowing to re-send request in case it's granted by the config file
	sendRequest func(govppapi.Message) govppapi.RequestCtx
	// Parameter for sendRequest
	requestMsg govppapi.Message
	// Retry data
	retry retryConfig
	// Tracer object
	tracer measure.Tracer
	// Start time
	start time.Time
}

// govppMultirequestCtx is custom govpp MultiRequestCtx.
type govppMultirequestCtx struct {
	// Original multi request context
	requestCtx govppapi.MultiRequestCtx
	// Parameter for sendRequest
	requestMsg govppapi.Message
	// Tracer object
	tracer measure.Tracer
	// Start time
	start time.Time
}

// SendRequest sends asynchronous request to the vpp and receives context used to receive reply.
// Plugin govppmux allows to re-send retry which failed because of disconnected vpp, if enabled.
func (c *goVppChan) SendRequest(request govppapi.Message) govppapi.RequestCtx {
	start := time.Now()

	sendRequest := c.Channel.SendRequest
	// Send request now and wait for context
	requestCtx := sendRequest(request)

	// Return context with value and function which allows to send request again if needed
	return &govppRequestCtx{
		requestCtx:  requestCtx,
		sendRequest: sendRequest,
		requestMsg:  request,
		retry:       c.retry,
		tracer:      c.tracer,
		start:       start,
	}
}

// ReceiveReply handles request and returns error if occurred. Also does retry if this option is available.
func (r *govppRequestCtx) ReceiveReply(reply govppapi.Message) error {
	defer func() {
		if r.tracer != nil {
			r.tracer.LogTime(r.requestMsg.GetMessageName(), r.start)
		}
	}()

	var timeout time.Duration
	maxAttempts := r.retry.attempts
	if r.retry.timeout > 0 { // Default value is 500ms
		timeout = r.retry.timeout
	}

	var err error
	// Receive reply from original send
	if err = r.requestCtx.ReceiveReply(reply); err == core.ErrNotConnected && maxAttempts > 0 {
		// Try to re-sent requests
		for attemptIdx := 1; attemptIdx <= maxAttempts; attemptIdx++ {
			// Wait, then try again
			time.Sleep(timeout)
			logrus.DefaultLogger().Warnf("Govppmux: retrying binary API message %v, attempt: %d",
				r.requestMsg.GetMessageName(), attemptIdx)
			if err = r.sendRequest(r.requestMsg).ReceiveReply(reply); err != core.ErrNotConnected {
				return err
			}
		}
	}

	return err
}

// SendMultiRequest sends asynchronous request to the vpp and receives context used to receive reply.
func (c *goVppChan) SendMultiRequest(request govppapi.Message) govppapi.MultiRequestCtx {
	start := time.Now()

	sendMultiRequest := c.Channel.SendMultiRequest
	// Send request now and wait for context
	requestCtx := sendMultiRequest(request)

	// Return context with value and function which allows to send request again if needed
	return &govppMultirequestCtx{
		requestCtx: requestCtx,
		requestMsg: request,
		tracer:     c.tracer,
		start:      start,
	}
}

// ReceiveReply handles request and returns error if occurred.
func (r *govppMultirequestCtx) ReceiveReply(reply govppapi.Message) (bool, error) {
	// Receive reply from original send
	last, err := r.requestCtx.ReceiveReply(reply)
	if last {
		defer func() {
			if r.tracer != nil {
				r.tracer.LogTime(r.requestMsg.GetMessageName(), r.start)
			}
		}()
	}
	return last, err
}
