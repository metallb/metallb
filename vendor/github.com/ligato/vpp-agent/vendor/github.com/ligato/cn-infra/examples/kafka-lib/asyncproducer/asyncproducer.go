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

	"github.com/namsral/flag"

	"strings"

	"github.com/ligato/cn-infra/examples/kafka-lib/utils"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/messaging/kafka/client"
	"github.com/ligato/cn-infra/utils/clienttls"
)

var (
	// Flags used to read the input arguments.
	brokerList    = flag.String("brokers", os.Getenv("KAFKA_PEERS"), "The comma separated list of brokers in the Kafka cluster. You can also set the KAFKA_PEERS environment variable")
	partitioner   = flag.String("partitioner", "hash", "The partitioning scheme to use. Can be `hash`, `manual`, or `random`")
	partition     = flag.Int("partition", -1, "The partition to produce to.")
	debug         = flag.Bool("debug", false, "turns on debug logging")
	tlsEnabled    = flag.Bool("tlsEnabled", false, "turns on TLS communication")
	tlsSkipVerify = flag.Bool("tlsSkipVerify", true, "skips verification of server name & certificate")
	tlsCAFile     = flag.String("tlsCAFile", "", "Certificate Authority")
	tlsCertFile   = flag.String("tlsCertFile", "", "Client Certificate")
	tlsKeyFile    = flag.String("tlsKeyFile", "", "Client Private Key")
)

func main() {
	flag.Parse()

	if *brokerList == "" {
		printUsageErrorAndExit("no -brokers specified. Alternatively, set the KAFKA_PEERS environment variable")
	}

	succCh := make(chan *client.ProducerMessage)
	errCh := make(chan *client.ProducerError)

	// init config
	config := client.NewConfig(logrus.DefaultLogger())
	config.SetDebug(*debug)
	config.SetPartition(int32(*partition))
	config.SetSendSuccess(true)
	config.SetSuccessChan(succCh)
	config.SetSendError(true)
	config.SetErrorChan(errCh)
	config.SetBrokers(strings.Split(*brokerList, ",")...)

	tls := clienttls.TLS{
		Enabled:    *tlsEnabled,
		SkipVerify: *tlsSkipVerify,
		CAfile:     *tlsCAFile,
		Certfile:   *tlsCertFile,
		Keyfile:    *tlsKeyFile,
	}

	tlsConfig, err := clienttls.CreateTLSConfig(tls)
	if err != nil {
		fmt.Printf("Failed to create TLS config: %v", err)
		os.Exit(1)
	}
	config.SetTLS(tlsConfig)

	sClient, err := client.NewClient(config, *partitioner)
	if err != nil {
		os.Exit(1)
	}

	// Create new Async-producer using NewAsyncProducer() API.
	producer, err := client.NewAsyncProducer(config, sClient, *partitioner, nil)
	if err != nil {
		fmt.Printf("NewAsyncProducer errored: %v\n", err)
		os.Exit(1)
	}

	// A separate go routine monitors the status of the message delivery.
	go func() {
	eventLoop:
		for {
			select {
			case <-producer.GetCloseChannel():
				break eventLoop
			case msg := <-succCh:
				fmt.Println("message sent successfully - ", msg)
			case err := <-errCh:
				fmt.Println("message errored - ", err)
			}
		}
	}()

	// Prompt user for the command to execute and read the input parameters.
	for {
		command := utils.GetCommand()
		switch command.Cmd {
		case "quit":
			// Exit the application.
			err := closeProducer(producer)
			if err != nil {
				fmt.Println("terminated abnormally")
				os.Exit(1)
			}
			fmt.Println("ended successfully")
			os.Exit(0)
		case "message":
			// Send the message.
			err := sendMessage(producer, command.Message)
			if err != nil {
				fmt.Printf("send message error: %v\n", err)
			}

		default:
			fmt.Println("invalid command")
		}
	}
}

// sendMessage demonstrates AsyncProducer.SendMsgByte API to publish a single
// message to a Kafka topic.
func sendMessage(producer *client.AsyncProducer, msg utils.Message) error {
	var (
		msgKey   []byte
		msgMeta  []byte
		msgValue []byte
	)

	// init message
	if msg.Key != "" {
		msgKey = []byte(msg.Key)
	}
	if msg.Metadata != "" {
		msgMeta = []byte(msg.Metadata)
	}
	msgValue = []byte(msg.Text)

	// send message
	producer.SendMsgByte(msg.Topic, msgKey, msgValue, msgMeta)

	fmt.Println("message sent")
	return nil
}

// closeProducer uses the AsyncProducer.Close() API to close the producer.
func closeProducer(producer *client.AsyncProducer) error {
	// close producer
	fmt.Println("closing producer ...")
	err := producer.Close(true)
	if err != nil {
		fmt.Printf("AsyncProducer close errored: %v\n", err)
		return err
	}
	return nil
}

func printUsageErrorAndExit(message string) {
	fmt.Fprintln(os.Stderr, "ERROR:", message)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Available command line options:")
	flag.PrintDefaults()
	os.Exit(64)
}
