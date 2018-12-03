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
	"fmt"
	"os"
	"os/signal"

	"time"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/messaging/kafka/client"
	"github.com/ligato/cn-infra/messaging/kafka/mux"
	"github.com/namsral/flag"
)

func main() {
	var configFile string

	// Get the config file name from the input arguments or from an environment
	// variable.
	logrus.DefaultLogger().SetLevel(logging.DebugLevel)
	flag.StringVar(&configFile, "config", "", "Configuration file path.")
	flag.Parse()

	// Initialize multiplexer named "default".
	mx, err := mux.InitMultiplexer(configFile, "default", logrus.DefaultLogger())
	if err != nil {
		fmt.Printf("Error initializing multiplexer %v", err)
		os.Exit(1)
	}

	// Create a new connection. This is a virtual connection, created on top
	// of the real connection from the multiplexer.
	cn := mx.NewBytesConnection("plugin")

	// Send one message synchronously.
	offset, err := cn.SendSyncString("test", "key", "value")
	if err == nil {
		fmt.Println("Sync published ", offset)
	}

	succCh := make(chan *client.ProducerMessage)
	errCh := make(chan *client.ProducerError)
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, os.Interrupt)

	// Send one message asynchronously.
	cn.SendAsyncString("test", "key", "async!!", "meta", mux.ToBytesProducerChan(succCh), mux.ToBytesProducerErrChan(errCh))

	// Receive the asynchronously send message.
	select {
	case success := <-succCh:
		fmt.Println("Successfully send async msg", success.Metadata)
	case err := <-errCh:
		fmt.Println("Error while sending async msg", err.Err, err.ProducerMessage.Metadata)
	}

	// Consume topic "test" for at most 3 seconds.
	consumerChan := make(chan *client.ConsumerMessage)
	err = cn.ConsumeTopic(mux.ToBytesMsgChan(consumerChan), "test")
	mx.Start()
	if err == nil {
		fmt.Println("Consuming test partition")
	eventLoop:
		for {
			select {
			case msg := <-consumerChan:
				fmt.Println(string(msg.Key), string(msg.Value))
			case <-time.After(3 * time.Second):
				break eventLoop
			case <-signalChan:
				break eventLoop
			}
		}
	}

	mx.Close()
}
