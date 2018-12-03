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

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/ligato/cn-infra/logging/logrus"
)

const loggerName = "loggerName"
const loggerLevel = "level"

var logRegistry = logrus.NewLogRegistry()
var defaultLogger = logrus.DefaultLogger()

// Every 10 seconds prints a set of logs using default and custom logger
func generateLogs() {
	myLogger := logRegistry.NewLogger("MyLogger")

	for range time.NewTicker(10 * time.Second).C {

		myLogger.Debug("My logger")
		myLogger.Info("My logger")
		myLogger.Error("My logger")

		defaultLogger.Debug("Default logger")
		defaultLogger.Info("Default logger")
		defaultLogger.Error("Default logger")
	}
}

func listLoggers(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(logRegistry.ListLoggers())
}

func setLevel(w http.ResponseWriter, r *http.Request) {
	encoder := json.NewEncoder(w)
	vars := mux.Vars(r)
	err := logRegistry.SetLevel(vars[loggerName], vars[loggerLevel])
	if err != nil {
		encoder.Encode(struct{ Err string }{err.Error()})
	} else {
		encoder.Encode(struct{ Status string }{"OK"})
	}
}

func main() {
	go generateLogs()

	// start http server to allow remote log level change in runtime
	//
	// To list all registered logger and current log level
	//  curl localhost:8080/list
	// To modify log level
	//  curl -X PUT localhost:8080/set/{loggerName}/{logLevel}
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/list", listLoggers).Methods("GET")
	router.HandleFunc(fmt.Sprintf("/set/{%s}/{%s:debug|info|warning|error|fatal|panic}", loggerName, loggerLevel), setLevel).Methods("PUT")
	err := http.ListenAndServe(":8080", router)
	if err != nil{
		defaultLogger.Fatal(err)
	}
}
