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

/*
Package kafka implements a client for the Kafka broker. The client supports
sending and receiving of messages through the Kafka message bus. It provides
both sync and async Producers for sending Kafka messages and a Consumer for
retrieving Kafka messages.

A Producer sends messages to Kafka. A Producer can be either synchronous
or asynchronous. Request to send a message using a synchronous producer
blocks until the message is published or an error is returned. A request
sent using asynchronous producer returns immediately and the success or
failure is communicated to the sender through a separate status channels.

A Consumer receives messages from Kafka for one or more topics. When a
consumer is initialized,it automatically balances/shares the total number
partitions for a message topic over all the active brokers for a topic.
Message offsets can optionally be committed to Kafka so that when a consumer
is restarted or a new consumer is initiated it knows where to begin reading
messages from the Kafka message log.

The package also provides a Multiplexer that allows to share consumer and
producers instances among multiple entities called Connections. Apart from
reusing the access to kafka brokers, the Multiplexer marks the offset of
consumed message as read. Offset marking allows to consume messages from the
last committed offset after the restart of the Multiplexer.

Note: Marking offset does not necessarily commit the offset to the backend
store immediately. This might result in a corner case where a message might
be delivered multiple times.

Usage of synchronous producer:
	// create minimal configuration
	config := client.NewConfig()
	config.SetBrokers("ip_addr:port", "ip_addr2:port")


	producer, err := client.NewSyncProducer(config, nil)
	if err != nil {
		os.Exit(1)
	}
	// key and value are of type []byte
	producer.SendMsgByte(topic, key, value, meta)

	// key and value are of type Encoder
	producer.SendMsgToPartition(topic, key, value, meta)

Usage of asynchronous producer:
	succCh := make(chan *client.ProducerMessage)
	errCh := make(chan *client.ProducerError)

	// init config
	config := client.NewConfig()
	config.SetSendSuccess(true)
	config.SetSuccessChan(succCh)
	config.SetSendError(true)
	config.SetErrorChan(errCh)
	config.SetBrokers("ip_addr:port", "ip_addr2:port")

	// init producer
	producer, err := client.NewAsyncProducer(config, nil)

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

	producer.SendMsgByte(topic, key, value, meta)

Usage of consumer:
        config := client.NewConfig()
	config.SetRecvNotification(true)
	config.SetRecvNotificationChan(make(chan *cluster.Notification))
	config.SetRecvError(true)
	config.SetRecvErrorChan(make(chan error))
	config.SetRecvMessageChan(make(chan *client.ConsumerMessage))
	config.SetBrokers("ip_addr:port", "ip_addr2:port2")
	config.SetTopics("topic1,topic2")
	config.SetGroup("Group1")

	consumer, err := client.NewConsumer(config, nil)
	if err != nil {
		log.Errorf("NewConsumer Error: %v", err)
		os.Exit(1)
	}

	go func() {
		for {
			select {
			case notification := <-config.RecvNotificationChan:
				handleNotifcation(consumer)
			case err := <-config.RecvErrorChan:
				fmt.Printf("Message Recv Errored: %v\n", err)
			case msg := <-config.RecvMessageChan:
				messageCallback(consumer, msg, *commit)
			case <-consumer.GetCloseChannel():
				return
			}
		}
	}()


In addition to basic sync/async producer and consumer the Multiplexer is provided. It's behaviour is depicted below:


 +---------------+              +--------------------+
 | Connection #1 |------+       |    Multiplexer     |
 +---------------+      |       |                    |
                        |       |    sync producer   |
 +---------------+      |       |   async producer   |		       /------------\
 | Connection #2 |------+-----> |    consumer        |<---------->/    Kafka     \
 +---------------+      |       |                    |            \--------------/
                        |       |                    |
 +---------------+      |       |                    |
 | Connection #3 |------+       +--------------------+
 +---------------+

To initialize multiplexer run:

   mx, err := mux.InitMultiplexer(pathToConfig, "name")

The config file specifies addresses of kafka brokers:
   addrs:
     - "ip_addr1:port"
     - "ip_addr2:port"

To create a Connection that reuses Multiplexer access to kafka run:

	cn := mx.NewBytesConnection("c1")

	or

	cn := mx.NewProtoConnection("c1")

Afterwards you can produce messages using sync API:

	partition, offset, err := cn.SendSyncString("test", "key", "value")

or you can use async API:
	succCh := make(chan *client.ProducerMessage, 10)
	errCh := make(chan *client.ProducerError, 10)
        cn.SendAsyncString("test", "key", "async message", "meta", succCh, errCh)

        // check if the async send succeeded
        go func() {
           select {
	   case success := <-succCh:
		fmt.Println("Successfully send async msg", success.Metadata)
	   case err := <-errCh:
		fmt.Println("Error while sending async msg", err.Err, err.Msg.Metadata)
	   }
	}()

subscribe to consume a topic:
	consumerChan := make(chan *client.ConsumerMessage
        err = cn.ConsumeTopic("test", consumerChan)

	if err == nil {
		fmt.Println("Consuming test partition")
		go func() {
			eventLoop:
				for {
					select {
					case msg := <-consumerChan:
						fmt.Println(string(msg.Key), string(msg.Value))
					case <-signalChan:
						break eventLoop
					}
				}
		}()
	}


Once all connection have subscribed for topic consumption. You have to run the following function
to actually initialize the consumer inside the Multiplexer.

        mx.Start()

To properly clean up the Multiplexer call:

        mx.Close()


The KAFKA plugin

Once kafka plugin is initialized
        plugin := kafka.Plugin{}
        // Init called by agent core

The plugin allows to create connections:

        conn := plugin.NewConnection("name")

or connection that support proto-modelled messages:

        protoConn := plugin.NewProtoConnection("protoConnection")
The usage of connections is described above.

*/
package kafka
