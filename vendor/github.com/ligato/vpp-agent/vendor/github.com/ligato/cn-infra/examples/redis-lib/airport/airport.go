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

//go:generate protoc --proto_path=./model --gogo_out=./model ./model/flight.proto
package main

import (
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/pkg/errors"

	"fmt"

	"sort"
	"strconv"

	"math"
	"sync/atomic"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/kvproto"
	"github.com/ligato/cn-infra/db/keyval/redis"
	"github.com/ligato/cn-infra/examples/redis-lib/airport/model"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/namsral/flag"
)

var diagram = `
                                                 =========
                                                  Airport
                                                 =========


         +---------+               +----------+
 put --->| Arrival |               |  Runway  |
         |---------|               |----------|
         |        --- put event -->|          |
         |         |<--- delete ---|          |                                        +-----------+
         +---------+               |  land    |                                        |  Hangar   |
                                   |          |< maintenance? > [yes] -- put w/ TTL -->|-----------|
                                   |          |     [no]                               | (expired) |
                                   |          |       |         +-----------+          +----|------+
                                   |          |       +- put -->| Departure |<-- put <-- del event
                                   |          |                 |-----------|
                                   |          |<--- put event ----          |
                                   |          |---- delete ---->|           |
                                   | take off |                 +-----------+
                                   +----------+

`

var flightStatusFormat = "\r"

// Labels
const (
	arrival   = "Arrival"
	departure = "Departure"
	runway    = "Runway"
	hangar    = "Hangar"
)

// Airport parameters
const (
	flightSlots = 5

	runwayLength    = 30
	runwayInterval  = 20000000
	runwayClearance = 400000000
	runwaySpeedBump = 0.5 // speed modifier

	hangarSlots        = 3
	hangarThreshold    = 0.5
	hangarDurationLow  = 2000000000
	hangarDurationHigh = 6000000000
	hangarKeyTemplate  = "%2s%02d:%d"
)

// Aircraft parameters
const (
	flightIDLength   = 5
	flightStatusSize = 2*flightSlots + hangarSlots + 1
	flightIDFormat   = "%s%02d"
)

// Other
const (
	columnSep = "      "
)

// Priority list of flights
type priorities []uint32

// Len method (in order to implement Sorted)
func (p priorities) Len() int { return len(p) }

// Swap method (in order to implement Sorted)
func (p priorities) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// Less method (in order to implement Sorted)
func (p priorities) Less(i, j int) bool { return p[i] < p[j] }

// Airport struct to manage arrivals/departures
type Airport struct {
	sync.Mutex
	log    *logrus.Logger
	client redis.Client

	connection *redis.BytesConnectionRedis

	// parameters
	airlines    []string
	flightRadar map[string]struct{}
	priority    uint32 // the order in which the flights arrive

	// prefixes
	arrivalPrefix   string
	departurePrefix string
	hangarPrefix    string

	// brokers
	arrivalBroker   keyval.ProtoBroker
	departureBroker keyval.ProtoBroker
	hangarBroker    keyval.ProtoBroker

	// watchers
	arrivalWatcher   keyval.ProtoWatcher
	departureWatcher keyval.ProtoWatcher
	hangarWatcher    keyval.ProtoWatcher

	// watch channels
	arrivalChan   chan datasync.ProtoWatchResp
	departureChan chan datasync.ProtoWatchResp
	hangarChan    chan datasync.ProtoWatchResp
	runwayChan    chan flight.Info

	// other
	motions   []string
	respChan  chan keyval.BytesWatchResp
	closeChan chan string
}

// Initialize airport and start serving
func main() {
	var debug bool
	var redisConfigPath string

	// init example flags
	flag.BoolVar(&debug, "debug", false, "Enable debugging")
	flag.StringVar(&redisConfigPath, "redis-config", "", "Redis configuration file path")
	flag.Parse()

	log := logrus.DefaultLogger()
	if debug {
		log.SetLevel(logging.DebugLevel)
	}
	// load redis config file
	redisConfig, err := redis.LoadConfig(redisConfigPath)
	if err != nil {
		log.Errorf("Failed to load Redis config file %s: %v", redisConfigPath, err)
		return
	}

	airport := &Airport{
		log:             log,
		airlines:        []string{"AA", "DL", "SW", "UA"},
		flightRadar:     make(map[string]struct{}),
		arrivalPrefix:   "/redis/airport/arrival",
		departurePrefix: "/redis/airport/departure",
		hangarPrefix:    "/redis/airport/hangar",
		motions:         []string{" ->", "<- "},
		respChan:        make(chan keyval.BytesWatchResp, 10),
		closeChan:       make(chan string),
	}
	doneChan := make(chan struct{})
	if err := airport.init(redisConfig, doneChan); err != nil {
		airport.log.Errorf("airport example error: %v", err)
	} else {
		airport.start()
	}
}

// Set all required brokers, watchers, prepare redis connection
func (a *Airport) init(config interface{}, doneChan chan struct{}) (err error) {
	a.log.Info("Airport redis example. If you need more info about what is happening, run example with -debug=true")

	rand.Seed(time.Now().UnixNano())

	printHeaders()
	setupFlightStatusFormat()

	// prepare client to connect to the redis DB
	a.client, err = redis.ConfigToClient(config)
	if err != nil {
		return fmt.Errorf("failed to create redis client: %v", err)
	}
	a.connection, err = redis.NewBytesConnection(a.client, a.log)
	if err != nil {
		return fmt.Errorf("failed to create connection from redis client: %v", err)
	}

	// prepare all the brokers and watchers in order to simulate airport
	a.arrivalBroker = kvproto.NewProtoWrapper(a.connection).NewBroker(a.arrivalPrefix)
	a.arrivalWatcher = kvproto.NewProtoWrapper(a.connection).NewWatcher(a.arrivalPrefix)

	a.departureBroker = kvproto.NewProtoWrapper(a.connection).NewBroker(a.departurePrefix)
	a.departureWatcher = kvproto.NewProtoWrapper(a.connection).NewWatcher(a.departurePrefix)

	a.hangarBroker = kvproto.NewProtoWrapper(a.connection).NewBroker(a.hangarPrefix)
	a.hangarWatcher = kvproto.NewProtoWrapper(a.connection).NewWatcher(a.hangarPrefix)

	a.cleanUp(false)

	// start watchers
	a.arrivalChan = make(chan datasync.ProtoWatchResp, flightSlots)
	if err := a.arrivalWatcher.Watch(keyval.ToChanProto(a.arrivalChan), nil, ""); err != nil {
		return fmt.Errorf("failed to start 'arrival' watcher: %v", err)
	}
	a.departureChan = make(chan datasync.ProtoWatchResp, flightSlots)
	if err := a.departureWatcher.Watch(keyval.ToChanProto(a.departureChan), nil, ""); err != nil {
		return fmt.Errorf("failed to start 'departure' watcher: %v", err)
	}
	a.hangarChan = make(chan datasync.ProtoWatchResp, hangarSlots)
	if err := a.hangarWatcher.Watch(keyval.ToChanProto(a.hangarChan), nil, ""); err != nil {
		return fmt.Errorf("failed to start 'hangar' watcher: %v", err)
	}
	a.runwayChan = make(chan flight.Info, flightSlots)

	return nil
}

// Start arrivals/departures and exit-on-signal procedure
func (a *Airport) start() {
	// start all the airport processors
	go a.startArrivals()
	go a.processArrivals()
	go a.processDepartures()
	go a.processHangar()

	// quit on os signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	for {
		select {
		case <-signalChan:
			a.cleanUp(true)
			if err := safeclose.Close(a.connection); err != nil {
				a.log.Error(err)
			}
			os.Exit(1)
		case runway, ok := <-a.runwayChan:
			if !ok {
				a.log.Errorf("runway channel closed")
			}
			a.processRunway(runway)
			time.Sleep(randomDuration(runwayClearance, runwayClearance))
		}
	}
}

// Generate 3 arrivals at start, then continue generating with random pause (1-5 seconds) in between
func (a *Airport) startArrivals() {
	for i := 0; i < flightSlots/2+1; i++ {
		if err := a.newArrival(); err != nil {
			a.log.Error(err)
		}
	}

	for {
		if err := a.newArrival(); err != nil {
			a.log.Error(err)
		}
		time.Sleep(randomDuration(1000000000, 5000000000))
	}
}

// Wait for arrivals. Incoming flights are set to 'arrival' status and sent to runway.
func (a *Airport) processArrivals() {
	for {
		arrival, ok := <-a.arrivalChan
		if !ok {
			a.log.Errorf("arrival channel closed")
			return
		}
		switch arrival.GetChangeType() {
		case datasync.Put:
			fl := flight.Info{}
			if err := arrival.GetValue(&fl); err != nil {
				a.log.Errorf("failed to get value for arrival flight: %v", err)
				continue
			}
			fl.Status = flight.Status_arrival
			a.runwayChan <- fl
		case datasync.Delete:
			a.log.Debugf("arrival %s deleted\n", arrival.GetKey())
		}
	}
}

// Wait for departures. Outgoing flights are set to 'departure' and sent to runway
func (a *Airport) processDepartures() {
	for {
		departure, ok := <-a.departureChan
		if !ok {
			a.log.Errorf("departure channel closed")
			return
		}
		switch departure.GetChangeType() {
		case datasync.Put:
			fl := flight.Info{}
			if err := departure.GetValue(&fl); err != nil {
				a.log.Errorf("failed to get value for departure flight: %v", err)
				continue
			}
			fl.Status = flight.Status_departure
			a.runwayChan <- fl
		case datasync.Delete:
			a.log.Debugf("departure %s deleted\n", departure.GetKey())
		}
	}
}

// Wait for hangar. Incoming flights are stored, outgoing are sent to departure.
func (a *Airport) processHangar() {
	for {
		hangar, ok := <-a.hangarChan
		if !ok {
			a.log.Errorf("hangar channel closed")
			return
		}
		switch hangar.GetChangeType() {
		case datasync.Put:
			a.log.Debugf("hangar %s updated", hangar.GetKey())
		case datasync.Delete:
			fl := flight.Info{}
			if _, err := fmt.Sscanf(hangar.GetKey(), hangarKeyTemplate, &(fl.Airline), &(fl.Number), &(fl.Priority)); err != nil {
				a.log.Errorf("error creating hangar key: %v", err)
				continue
			}
			if err := a.departureBroker.Put(fmt.Sprintf(flightIDFormat, fl.Airline, fl.Number), &fl); err != nil {
				a.log.Errorf("failed to put flight to departure broker: %v", err)
				continue
			}
		}
	}
}

// Runway can serve one flight at a time. If flight status is 'arrival', the plane lands and is either sent
// departure or to hangar (random chance)
func (a *Airport) processRunway(fl flight.Info) {
	flightID := fmt.Sprintf(flightIDFormat, fl.Airline, fl.Number)

	if fl.Status == flight.Status_arrival {
		a.log.Debugf("%s%s approaching runway\n", flightID, a.motions[fl.Status])
		_, err := a.arrivalBroker.Delete(flightID)
		if err != nil {
			a.log.Errorf("processRunway(%s) failed: %s", flightID, err)
		}
		a.land(fl)

		// send to departure or hangar
		if rand.Float64() <= hangarThreshold {
			err = a.hangarBroker.Put(makeHangarKey(fl), &fl, datasync.WithTTL(randomDuration(hangarDurationLow, hangarDurationHigh)))
		} else {
			err = a.departureBroker.Put(flightID, &fl)
		}
		if err != nil {
			a.log.Errorf("processRunway(%s) failed: %s", flightID, err)
		}
	} else {
		a.log.Debugf("%s%s approaching runway\n", flightID, a.motions[fl.Status])
		_, err := a.departureBroker.Delete(flightID)
		if err != nil {
			a.log.Errorf("processRunway(%s) failed: %s", flightID, err)
		}
		a.takeOff(fl)
		a.Lock()
		delete(a.flightRadar, flightID)
		a.Unlock()
	}
}

// Land the flight, calculate steps with decreasing size (imitating landing speed)
func (a *Airport) land(fl flight.Info) {
	flightInMotion := fmt.Sprintf(flightIDFormat, fl.Airline, fl.Number) + a.motions[fl.Status]
	size := len(flightInMotion)
	steps := runwayLength - size + 1
	interval := runwayInterval / 2
	var flightStatus = make([]interface{}, flightStatusSize)
	for i := 0; i < steps; i++ {
		flightStatus[flightSlots] = fmt.Sprintf("%*s", size+i, flightInMotion)
		a.fillArrivalStatus(flightStatus)
		a.fillDepartureStatus(flightStatus)
		a.fillHangarStatus(flightStatus)
		fmt.Printf(flightStatusFormat, flightStatus...)
		time.Sleep(randomDuration(interval, interval))
		if i >= int(float64(steps)*runwaySpeedBump) {
			interval += runwayInterval
		}
	}
}

// Flight takeoff, calculate steps with increasing size (imitating takeoff speed)
func (a *Airport) takeOff(fl flight.Info) {
	flightInMotion := a.motions[fl.Status] + fmt.Sprintf(flightIDFormat, fl.Airline, fl.Number)
	steps := runwayLength - len(flightInMotion) + 1
	interval := runwayInterval/2 + runwayInterval*math.Floor(float64(steps)*runwaySpeedBump)
	var flightStatus = make([]interface{}, flightStatusSize)
	for i := 0; i < steps; i++ {
		flightStatus[flightSlots] = fmt.Sprintf("%*s", runwayLength-i, flightInMotion)
		a.fillArrivalStatus(flightStatus)
		a.fillDepartureStatus(flightStatus)
		a.fillHangarStatus(flightStatus)
		fmt.Printf(flightStatusFormat, flightStatus...)
		time.Sleep(randomDuration(int(interval), int(interval)))
		if i < int(float64(steps)*runwaySpeedBump) {
			interval -= runwayInterval
		}
	}
}

// Generate new arrival for one of the airlines with some number (everything is chosen randomly)
func (a *Airport) newArrival() error {
	priority := atomic.AddUint32(&a.priority, 1)
	flightInfo := &flight.Info{
		Airline:  a.airlines[rand.Int()%len(a.airlines)],
		Number:   rand.Uint32()%99 + 1,
		Priority: priority,
	}
	var exists bool
	flightID := fmt.Sprintf(flightIDFormat, flightInfo.Airline, flightInfo.Number)

	a.Lock()
	// make sure that the generated flight does not exist yet and "show" it on the flight radar
	if _, exists = a.flightRadar[flightID]; !exists {
		a.flightRadar[flightID] = struct{}{}
		if err := a.arrivalBroker.Put(flightID, flightInfo); err != nil {
			return errors.Errorf("Arrival %s failed: %v", flightID, err)
		}
	}
	a.Unlock()

	return nil
}

// Auxiliary methods

func (a *Airport) cleanUp(report bool) {
	if report {
		a.log.Info("cleaning up airport")
		a.printFlightCounts()
	}
	if _, err := a.arrivalBroker.Delete("", datasync.WithPrefix()); err != nil {
		a.log.Errorf("failed to clean up arrivals: %v", err)
	}
	if _, err := a.departureBroker.Delete("", datasync.WithPrefix()); err != nil {
		a.log.Errorf("failed to clean up departures: %v", err)
	}
	if _, err := a.hangarBroker.Delete("", datasync.WithPrefix()); err != nil {
		a.log.Errorf("failed to clean up hangar: %v", err)
	}
	if report {
		a.printFlightCounts()
	}
}

func (a *Airport) printFlightCounts() {
	arrivals := countFlights(a.arrivalBroker, arrival)
	departures := countFlights(a.departureBroker, departure)
	hangars, err := a.getHangarFlights()
	if err != nil {
		a.log.Errorf("printFlightCounts() failed: %s", err)
	}
	fmt.Printf("arrivals %d, departures %d, hangars %d\n", arrivals, departures, len(hangars))
}

func (a *Airport) getHangarFlights() ([]flight.Info, error) {
	keys, err := a.hangarBroker.ListKeys("")
	if err != nil {
		return nil, fmt.Errorf("getHangarFlights() failed: %s", err)
	}

	var flights []flight.Info
	for {
		k, _, last := keys.GetNext()
		if last {
			break
		}
		f := flight.Info{}
		if err := scanHangarKey(k, &f); err != nil {
			a.log.Error(err)
		}
		flights = append(flights, f)
	}
	return flights, nil
}

func (a *Airport) fillArrivalStatus(flightStatus []interface{}) {
	arrivals, err := getSortedFlights(a.arrivalBroker, arrival)
	if err != nil {
		a.log.Errorf("fillArrivalStatus() failed: %s", err)
		return
	}
	for i := 0; i < flightSlots; i++ {
		flightStatus[i] = ""
	}

	count := len(arrivals)
	if count > 0 {
		if count > flightSlots {
			count = flightSlots
		}
		for i := 0; i < count; i++ {
			flightStatus[flightSlots-1-i] = fmt.Sprintf(flightIDFormat, arrivals[i].Airline, arrivals[i].Number)
		}
	}
}

func (a *Airport) fillDepartureStatus(flightStatus []interface{}) {
	departures, err := getSortedFlights(a.departureBroker, departure)
	if err != nil {
		a.log.Errorf("fillDepartureStatus() failed: %s", err)
		return
	}

	for i := flightSlots + 1; i < flightSlots*2+1; i++ {
		flightStatus[i] = ""
	}

	count := len(departures)
	if count > 0 {
		if count > flightSlots {
			count = flightSlots
		}
		for i := 0; i < count; i++ {
			flightStatus[flightSlots+1+i] = fmt.Sprintf(flightIDFormat, departures[i].Airline, departures[i].Number)
		}
	}
}

func (a *Airport) fillHangarStatus(flightStatus []interface{}) {
	hangars, err := a.getHangarFlights()
	if err != nil {
		a.log.Errorf("fillHangarStatus() failed: %s", err)
		return
	}

	for i := flightSlots*2 + 1; i < flightStatusSize; i++ {
		flightStatus[i] = ""
	}

	count := len(hangars)
	if count > 0 {
		if count > hangarSlots {
			count = hangarSlots
		}
		for i := 0; i < count; i++ {
			flightStatus[flightSlots*2+1+i] = fmt.Sprintf(flightIDFormat, hangars[i].Airline, hangars[i].Number)
		}
	}
}

func printHeaders() {
	fmt.Println()
	fmt.Print(diagram)
	fmt.Println()
	fmt.Println()

	diff := runwayLength - len(runway)
	pad := diff / 2
	var pad2 int
	if diff%2 == 0 {
		pad2 = pad
	} else {
		pad2 = pad + 1
	}
	fmt.Printf("%*s%s%*s%s%*s%s%-*s%s%s\n",
		flightIDLength*flightSlots, arrival,
		columnSep, pad, "", runway, pad2, "",
		columnSep, flightIDLength*flightSlots, departure,
		columnSep, hangar)
	dash60 := "-----------------------------------------------------------"
	waitingGuide := dash60[0 : flightIDLength*flightSlots]
	runwayGuide := dash60[0:runwayLength]
	hangarGuide := dash60[0 : flightIDLength*hangarSlots]
	fmt.Printf("%s%s%s%s%s%s%s\n",
		waitingGuide, columnSep, runwayGuide, columnSep, waitingGuide, columnSep, hangarGuide)
}

func setupFlightStatusFormat() {
	size := strconv.Itoa(flightIDLength)
	for i := 0; i < flightSlots; i++ {
		flightStatusFormat += "%" + size + "s"
	}
	flightStatusFormat += columnSep + "%-" + strconv.Itoa(runwayLength) + "s" + columnSep
	for i := 0; i < flightSlots; i++ {
		flightStatusFormat += "%-" + size + "s"
	}
	flightStatusFormat += columnSep
	for i := 0; i < hangarSlots; i++ {
		flightStatusFormat += "%-" + size + "s"
	}
}

func makeHangarKey(f flight.Info) string {
	return fmt.Sprintf(hangarKeyTemplate, f.Airline, f.Number, f.Priority)
}

func scanHangarKey(key string, f *flight.Info) error {
	if _, err := fmt.Sscanf(key, hangarKeyTemplate, &(f.Airline), &(f.Number), &(f.Priority)); err != nil {
		return err
	}
	return nil
}

func countFlights(broker keyval.ProtoBroker, name string) int {
	flights, err := getSortedFlights(broker, name)
	if err != nil {
		return 0
	}
	return len(flights)
}

func getSortedFlights(broker keyval.ProtoBroker, name string) ([]flight.Info, error) {
	kvi, err := broker.ListValues("")
	if err != nil {
		return nil, fmt.Errorf("getSortedFlights(%s) failed: %s", name, err)
	}
	var kvMap = make(map[uint32]flight.Info)
	var priorities = priorities{}
	for {
		kv, done := kvi.GetNext()
		if done {
			break
		}
		f := flight.Info{}
		if err := kv.GetValue(&f); err != nil {
			continue
		}
		priorities = append(priorities, f.Priority)
		kvMap[f.Priority] = f
	}

	if len(priorities) == 0 {
		return []flight.Info{}, nil
	}
	sort.Sort(priorities)
	var flights = make([]flight.Info, len(priorities))
	for i, k := range priorities {
		flights[i] = kvMap[k]
	}
	return flights, nil
}

func randomDuration(lowSecondsNs, highSecondsNs int) time.Duration {
	if highSecondsNs != lowSecondsNs {
		return time.Duration(rand.Intn(highSecondsNs-lowSecondsNs) + lowSecondsNs)
	}
	return time.Duration(lowSecondsNs)
}
