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

package kvscheduler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unrolled/render"

	"github.com/ligato/cn-infra/rpc/rest"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/utils"
	"net/url"
	"github.com/ligato/vpp-agent/plugins/kvscheduler/internal/graph"
	"sort"
)

const (
	// prefix used for REST urls of the KVScheduler.
	urlPrefix = "/scheduler/"

	// txnHistoryURL is URL used to obtain the transaction history.
	txnHistoryURL = urlPrefix + "txn-history"

	// sinceArg is the name of the argument used to define the start of the time
	// window for the transaction history to display.
	sinceArg = "since"

	// untilArg is the name of the argument used to define the end of the time
	// window for the transaction history to display.
	untilArg = "until"

	// seqNumArg is the name of the argument used to define the sequence number
	// of the transaction to display (txnHistoryURL).
	seqNumArg = "seq-num"

	// formatArg is the name of the argument used to set the output format
	// for the transaction history API.
	formatArg = "format"

	// recognized formats:
	formatJSON = "json"
	formatText = "text"

	// keyTimelineURL is URL used to obtain timeline of value changes for a given key.
	keyTimelineURL = urlPrefix + "key-timeline"

	// keyArg is the name of the argument used to define key for "key-timeline" API.
	keyArg = "key"

	// graphSnapshotURL is URL used to obtain graph snapshot from a given point in time.
	graphSnapshotURL = urlPrefix + "graph-snapshot"

	// flagStatsURL is URL used to obtain flag statistics.
	flagStatsURL = urlPrefix + "flag-stats"

	// flagArg is the name of the argument used to define flag for "flag-stats" API.
	flagArg = "flag"

	// prefixArg is the name of the argument used to define prefix to filter keys
	// for "flag-stats" API.
	prefixArg = "prefix"

	// time is the name of the argument used to define point in time for a graph snapshot
	// to retrieve.
	timeArg = "time"

	// downstreamResyncURL is URL used to trigger downstream-resync.
	downstreamResyncURL = urlPrefix + "downstream-resync"

	// retryArg is the name of the argument used for "downstream-resync" API to tell whether
	// to retry failed operations or not.
	retryArg = "retry"

	// verboseArg is the name of the argument used for "downstream-resync" API
	// to tell whether the refreshed graph should be printed to stdout or not.
	verboseArg = "verbose"

	// dumpURL is URL used to dump either SB or scheduler's internal state of kv-pairs
	// under the given descriptor / key-prefix.
	dumpURL = urlPrefix + "dump"

	// descriptorArg is the name of the argument used to define descriptor for "dump" API.
	descriptorArg = "descriptor"

	// keyPrefixArg is the name of the argument used to define key prefix for "dump" API.
	keyPrefixArg = "key-prefix"

	// viewArg is the name of the argument used for "dump" API to chooses from
	// which point of view to look at the key-value space when dumping values.
	// See type View from kvscheduler's API to learn the set of possible values.
	viewArg = "view"

	// statusURL is URL used to print the state of values under the given
	// descriptor / key-prefix or all of them.
	statusURL = urlPrefix + "status"
)

// errorString wraps string representation of an error that, unlike the original
// error, can be marshalled.
type errorString struct {
	Error string
}

// dumpIndex defines "index" page for the Dump REST API.
type dumpIndex struct {
	Descriptors []string
	KeyPrefixes []string
	Views       []string
}

// kvsWithMetaForREST converts a list of key-value pairs with metadata
// into an equivalent list with proto.Message recorded for proper marshalling.
func kvsWithMetaForREST(in []kvs.KVWithMetadata) (out []kvs.KVWithMetadata) {
	for _, kv := range in {
		out = append(out, kvs.KVWithMetadata{
			Key:      kv.Key,
			Value:    utils.RecordProtoMessage(kv.Value),
			Metadata: kv.Metadata,
			Origin:   kv.Origin,
		})
	}
	return out
}

// registerHandlers registers all supported REST APIs.
func (s *Scheduler) registerHandlers(http rest.HTTPHandlers) {
	if http == nil {
		s.Log.Warn("No http handler provided, skipping registration of KVScheduler REST handlers")
		return
	}
	http.RegisterHTTPHandler(txnHistoryURL, s.txnHistoryGetHandler, "GET")
	http.RegisterHTTPHandler(keyTimelineURL, s.keyTimelineGetHandler, "GET")
	http.RegisterHTTPHandler(graphSnapshotURL, s.graphSnapshotGetHandler, "GET")
	http.RegisterHTTPHandler(flagStatsURL, s.flagStatsGetHandler, "GET")
	http.RegisterHTTPHandler(downstreamResyncURL, s.downstreamResyncPostHandler, "POST")
	http.RegisterHTTPHandler(dumpURL, s.dumpGetHandler, "GET")
	http.RegisterHTTPHandler(statusURL, s.statusGetHandler, "GET")
	http.RegisterHTTPHandler(urlPrefix+"graph", s.dotGraphHandler, "GET")
}

// txnHistoryGetHandler is the GET handler for "txn-history" API.
func (s *Scheduler) txnHistoryGetHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var since, until time.Time
		var seqNum uint64
		args := req.URL.Query()

		// parse optional *format* argument (default = JSON)
		format := formatJSON
		if formatStr, withFormat := args[formatArg]; withFormat && len(formatStr) == 1 {
			format = formatStr[0]
			if format != formatJSON && format != formatText {
				err := errors.New("unrecognized output format")
				formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()})
				return
			}
		}

		// parse optional *seq-num* argument
		if seqNumStr, withSeqNum := args[seqNumArg]; withSeqNum && len(seqNumStr) == 1 {
			var err error
			seqNum, err = strconv.ParseUint(seqNumStr[0], 10, 64)
			if err != nil {
				s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
				return
			}

			// sequence number takes precedence over the since-until time window
			txn := s.GetRecordedTransaction(seqNum)
			if txn == nil {
				err := errors.New("transaction with such sequence number is not recorded")
				s.logError(formatter.JSON(w, http.StatusNotFound, errorString{err.Error()}))
				return
			}

			if format == formatJSON {
				s.logError(formatter.JSON(w, http.StatusOK, txn))
			} else {
				s.logError(formatter.Text(w, http.StatusOK, txn.StringWithOpts(false, true,0)))
			}
			return
		}

		// parse optional *until* argument
		if untilStr, withUntil := args[untilArg]; withUntil && len(untilStr) == 1 {
			var err error
			until, err = stringToTime(untilStr[0])
			if err != nil {
				s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
				return
			}
		}

		// parse optional *since* argument
		if sinceStr, withSince := args[sinceArg]; withSince && len(sinceStr) == 1 {
			var err error
			since, err = stringToTime(sinceStr[0])
			if err != nil {
				s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
				return
			}
		}

		txnHistory := s.GetTransactionHistory(since, until)
		if format == formatJSON {
			s.logError(formatter.JSON(w, http.StatusOK, txnHistory))
		} else {
			s.logError(formatter.Text(w, http.StatusOK, txnHistory.StringWithOpts(false, false,0)))
		}
	}
}

// keyTimelineGetHandler is the GET handler for "key-timeline" API.
func (s *Scheduler) keyTimelineGetHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		args := req.URL.Query()

		// parse mandatory *key* argument
		if keys, withKey := args[keyArg]; withKey && len(keys) == 1 {
			graphR := s.graph.Read()
			defer graphR.Release()

			timeline := graphR.GetNodeTimeline(keys[0])
			s.logError(formatter.JSON(w, http.StatusOK, timeline))
			return
		}

		err := errors.New("missing key argument")
		s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
	}
}

// graphSnapshotGetHandler is the GET handler for "graph-snapshot" API.
func (s *Scheduler) graphSnapshotGetHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		timeVal := time.Now()
		args := req.URL.Query()

		// parse optional *time* argument
		if timeStr, withTime := args[timeArg]; withTime && len(timeStr) == 1 {
			var err error
			timeVal, err = stringToTime(timeStr[0])
			if err != nil {
				s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
				return
			}
		}

		graphR := s.graph.Read()
		defer graphR.Release()

		snapshot := graphR.GetSnapshot(timeVal)
		s.logError(formatter.JSON(w, http.StatusOK, snapshot))
	}
}

// flagStatsGetHandler is the GET handler for "flag-stats" API.
func (s *Scheduler) flagStatsGetHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		args := req.URL.Query()

		// parse repeated *prefix* argument
		prefixes := args[prefixArg]

		if flags, withFlag := args[flagArg]; withFlag && len(flags) == 1 {
			graphR := s.graph.Read()
			defer graphR.Release()

			stats := graphR.GetFlagStats(flags[0], func(key string) bool {
				if len(prefixes) == 0 {
					return true
				}
				for _, prefix := range prefixes {
					if strings.HasPrefix(key, prefix) {
						return true
					}
				}
				return false
			})
			s.logError(formatter.JSON(w, http.StatusOK, stats))
			return
		}

		err := errors.New("missing flag argument")
		s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
	}
}

// downstreamResyncPostHandler is the POST handler for "downstream-resync" API.
func (s *Scheduler) downstreamResyncPostHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// parse optional *retry* argument
		args := req.URL.Query()
		retry := false
		if retryStr, withRetry := args[retryArg]; withRetry && len(retryStr) == 1 {
			retryVal := retryStr[0]
			if retryVal == "true" || retryVal == "1" {
				retry = true
			}
		}

		// parse optional *verbose* argument
		verbose := false
		if verboseStr, withVerbose := args[verboseArg]; withVerbose && len(verboseStr) == 1 {
			verboseVal := verboseStr[0]
			if verboseVal == "true" || verboseVal == "1" {
				verbose = true
			}
		}

		ctx := context.Background()
		ctx = kvs.WithResync(ctx, kvs.DownstreamResync, verbose)
		if retry {
			ctx = kvs.WithRetryDefault(ctx)
		}
		_, err := s.StartNBTransaction().Commit(ctx)
		if err != nil {
			s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
			return
		}
		s.logError(formatter.Text(w, http.StatusOK, "SB was successfully synchronized with KVScheduler\n"))
	}
}

func parseDumpAndStatusCommonArgs(args url.Values) (descriptor, keyPrefix string, err error) {
	// parse optional *descriptor* argument
	descriptors, withDescriptor := args[descriptorArg]
	if withDescriptor && len(descriptors) != 1 {
		err = errors.New("descriptor argument listed more than once")
		return
	}
	descriptor = descriptors[0]

	// parse optional *key-prefix* argument
	keyPrefixes, withKeyPrefix := args[keyPrefixArg]
	if withKeyPrefix && len(keyPrefixes) != 1 {
		err = errors.New("key-prefix argument listed more than once")
		return
	}
	keyPrefix = keyPrefixes[0]
	return
}

// dumpGetHandler is the GET handler for "dump" API.
func (s *Scheduler) dumpGetHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		args := req.URL.Query()

		descriptor, keyPrefix, err := parseDumpAndStatusCommonArgs(args)
		if err != nil {
			s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
			return
		}

		// without descriptor and key prefix return "index" page
		if descriptor == "" && keyPrefix == "" {
			s.txnLock.Lock()
			defer s.txnLock.Unlock()
			index := dumpIndex{Views: []string{
				kvs.SBView.String(), kvs.NBView.String(), kvs.InternalView.String()}}
			for _, descriptor := range s.registry.GetAllDescriptors() {
				index.Descriptors = append(index.Descriptors, descriptor.Name)
				index.KeyPrefixes = append(index.KeyPrefixes, descriptor.NBKeyPrefix)
			}
			s.logError(formatter.JSON(w, http.StatusOK, index))
			return
		}

		// parse optional *view* argument (default = SBView)
		var view kvs.View
		if viewStr, withState := args[viewArg]; withState && len(viewStr) == 1 {
			switch viewStr[0] {
			case kvs.SBView.String():
				view = kvs.SBView
			case kvs.NBView.String():
				view = kvs.NBView
			case kvs.InternalView.String():
				view = kvs.InternalView
			default:
				err := errors.New("unrecognized system view")
				s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
				return
			}
		}

		var dump []kvs.KVWithMetadata
		if descriptor != "" {
			dump, err = s.DumpValuesByDescriptor(descriptor, view)
		} else {
			dump, err = s.DumpValuesByKeyPrefix(keyPrefix, view)
		}
		if err != nil {
			s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
			return
		}
		s.logError(formatter.JSON(w, http.StatusOK, kvsWithMetaForREST(dump)))
	}
}

// statusGetHandler is the GET handler for "status" API.
func (s *Scheduler) statusGetHandler(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		args := req.URL.Query()

		descriptor, keyPrefix, err := parseDumpAndStatusCommonArgs(args)
		if err != nil {
			s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
			return
		}

		graphR := s.graph.Read()
		defer graphR.Release()

		if descriptor == "" && keyPrefix != "" {
			descriptor = s.getDescriptorForKeyPrefix(keyPrefix)
			if descriptor == "" {
				err = errors.New("unknown key prefix")
			}
			s.logError(formatter.JSON(w, http.StatusInternalServerError, errorString{err.Error()}))
			return
		}

		var nodes []graph.Node
		if descriptor == "" {
			// get all nodes with base values
			nodes = graphR.GetNodes(nil, graph.WithoutFlags(&DerivedFlag{}))
		} else {
			// get nodes with base values under the given descriptor
			nodes = graphR.GetNodes(nil,
				graph.WithFlags(&DescriptorFlag{descriptor}),
				graph.WithoutFlags(&DerivedFlag{}))
		}

		var status []*kvs.BaseValueStatus
		for _, node := range nodes {
			status = append(status, getValueStatus(node, node.GetKey()))
		}
		// sort by keys
		sort.Slice(status, func(i, j int) bool {
			return status[i].Value.Key < status[j].Value.Key
		})
		s.logError(formatter.JSON(w, http.StatusOK, status))
	}
}

// logError logs non-nil errors from JSON formatter
func (s *Scheduler) logError(err error) {
	if err != nil {
		s.Log.Error(err)
	}
}

// stringToTime converts Unix timestamp from string to time.Time.
func stringToTime(s string) (time.Time, error) {
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0), nil
}
