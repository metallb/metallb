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
	"strings"

	"github.com/namsral/flag"

	"github.com/ligato/cn-infra/examples/kafka-lib/utils"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/messaging/kafka/client"
	"github.com/ligato/cn-infra/utils/clienttls"
)

var (
	// Flags used to read the input arguments.
	brokerList    = flag.String("brokers", os.Getenv("KAFKA_PEERS"), "The comma separated list of brokers in the Kafka cluster. You can also set the KAFKA_PEERS environment variable")
	partitioner   = flag.String("partitioner", "hash", "The partitioning scheme to use. Can be `hash`, `manual`, or `random`")
	partition     = flag.Int("partition", -1, "The partition to produce to.")
	debug         = flag.Bool("debug", false, "turn on debug logging")
	tlsEnabled    = flag.Bool("tlsEnabled", false, "turns on TLS communication")
	tlsSkipVerify = flag.Bool("tlsSkipVerify", true, "skips verification of server name & certificate")
	tlsCAFile     = flag.String("tlsCAFile", "", "Certificate Authority")
	tlsCertFile   = flag.String("tlsCertFile", "", "Client Certificate")
	tlsKeyFile    = flag.String("tlsKeyFile", "", "Client Private Key")
)

func main() {
	logrus.DefaultLogger().SetLevel(logging.DebugLevel)
	flag.Parse()

	if *brokerList == "" {
		printUsageErrorAndExit("no -brokers specified. Alternatively, set the KAFKA_PEERS environment variable")
	}

	// init config
	config := client.NewConfig(logrus.DefaultLogger())
	config.SetDebug(*debug)
	config.SetPartition(int32(*partition))
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

	// Create new Sync-producer using NewSyncProducer() API.
	producer, err := client.NewSyncProducer(config, sClient, *partitioner, nil)
	if err != nil {
		fmt.Printf("NewSyncProducer errored: %v\n", err)
		os.Exit(1)
	}

	// Prompt user for the command to execute and read the input parameters.
	for {
		command := utils.GetCommand()
		switch command.Cmd {
		case "quit":
			err := closeProducer(producer)
			if err != nil {
				fmt.Println("terminated abnormally")
				os.Exit(1)
			}
			fmt.Println("ended successfully")
			os.Exit(0)
		case "message":
			err := sendMessage(producer, command.Message)
			if err != nil {
				fmt.Printf("send message error: %v\n", err)
			}

		default:
			fmt.Println("invalid command")
		}
	}
}

// sendMessage demonstrates SyncProducer.SendMsgByte API to publish a single
// message to a Kafka topic.
func sendMessage(producer *client.SyncProducer, msg utils.Message) error {
	var (
		msgKey   []byte
		msgValue []byte
	)

	// init message
	if msg.Key != "" {
		msgKey = []byte(msg.Key)
	}
	msgValue = []byte(msg.Text)

	// Demonstrate the use of SyncProducer.SendMsgByte() API.
	// The function doesn't return until the delivery status is known.
	_, err := producer.SendMsgByte(msg.Topic, msgKey, msgValue)
	if err != nil {
		logrus.DefaultLogger().Errorf("SendMsg Error: %v", err)
		return err
	}
	fmt.Println("message sent")
	return nil
}

func closeProducer(producer *client.SyncProducer) error {
	// close producer
	fmt.Println("Closing producer ...")
	err := producer.Close()
	if err != nil {
		fmt.Printf("SyncProducer close errored: %v\n", err)
		logrus.DefaultLogger().Errorf("SyncProducer close errored: %v", err)
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
