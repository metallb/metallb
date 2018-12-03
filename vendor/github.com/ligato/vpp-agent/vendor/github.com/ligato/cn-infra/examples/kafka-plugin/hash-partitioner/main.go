package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ligato/cn-infra/examples/model"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/messaging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/namsral/flag"
	"github.com/ligato/cn-infra/messaging/kafka"
	"github.com/ligato/cn-infra/agent"
	"log"
)

//********************************************************************
// The following functions show how to use the Agent's Kafka APIs
// to perform synchronous/asynchronous calls and how to watch on
// these events.
//********************************************************************

var (
	// Flags used to read the input arguments. Applies for both, sync and async message
	messageCount = flag.String("messageCount", os.Getenv("MSG_COUNT"), "Number of messages which will be send. Set to '0' to just watch")
)

// PluginName represents name of plugin.
const PluginName = "kafka-hash-example"

func main() {
	// Init example plugin and its dependencies
	ep := &ExamplePlugin{
		Deps: Deps{
			Log:          logging.ForPlugin(PluginName),
			Kafka:         &kafka.DefaultPlugin,
		},
		exampleFinished: make(chan struct{}),
	}
	// Start Agent with example plugin including dependencies
	a := agent.NewAgent(
		agent.AllPlugins(ep),
		agent.QuitOnClose(ep.exampleFinished),
	)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}

// ExamplePlugin demonstrates the use of Kafka plugin API from another plugin.
// The Kafka ConsumerHandle is required to read messages from a topic
// and PluginConnection is needed to start consuming on that topic.
type ExamplePlugin struct {
	Deps // plugin dependencies are injected

	subscription        chan messaging.ProtoMessage
	kafkaSyncPublisher  messaging.ProtoPublisher
	kafkaAsyncPublisher messaging.ProtoPublisher
	kafkaWatcher        messaging.ProtoWatcher
	// Successfully published kafka message is sent through the message channel.
	// In case of a failure it sent through the error channel.
	asyncSubscription   chan messaging.ProtoMessage
	asyncSuccessChannel chan messaging.ProtoMessage
	asyncErrorChannel   chan messaging.ProtoMessageErr

	// Fields below are used to properly finish the example.
	messagesSent bool
	syncRecv     bool
	asyncRecv    bool
	asyncSuccess bool
	exampleFinished chan struct{}
}

// Deps lists dependencies of ExamplePlugin.
type Deps struct {
	Kafka messaging.Mux // injected
	//local.PluginLogDeps               // injected
	Log logging.PluginLogger
}

// Consts
const (
	topic1     = "example-sync-topic"
	topic2     = "example-async-topic"
	connection = "example-proto-connection"
	subscriber = "example-part-watcher"
)

// These vars are applied for both, sync and async case
var (
	// How many messages will be sent
	messageCountNum = 10
)

// String returns plugin name
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

// Init initializes and starts producers and consumers.
func (plugin *ExamplePlugin) Init() (err error) {
	// handle flags
	flag.Parse()
	// message count flag
	if *messageCount != "" {
		messageCountNum, err = resolveMsgCount(*messageCount)
		if err != nil {
			return fmt.Errorf("'messageCount' has to be a number, not %v", *messageCount)
		}
		if messageCountNum < 0 {
			plugin.Log.Warnf("'messageCount' %v is not a positive number, defaulting to 0")
			messageCountNum = 0
		}
	} else {
		plugin.Log.Info("messageCount arg not set, using default value")
	}

	plugin.Log.Infof("Message count: %v", *messageCount)

	// Init channels required for async handler.
	plugin.asyncSuccessChannel = make(chan messaging.ProtoMessage)
	plugin.asyncErrorChannel = make(chan messaging.ProtoMessageErr)

	// Create a synchronous publisher for the selected topic.
	plugin.kafkaSyncPublisher, err = plugin.Kafka.NewSyncPublisher(connection, topic1)
	if err != nil {
		return err
	}

	// Create an asynchronous publisher for the selected topic.
	plugin.kafkaAsyncPublisher, err = plugin.Kafka.NewAsyncPublisher(connection, topic2, messaging.ToProtoMsgChan(plugin.asyncSuccessChannel),
		messaging.ToProtoMsgErrChan(plugin.asyncErrorChannel))
	if err != nil {
		return err
	}

	// Initialize sync watcher.
	plugin.kafkaWatcher = plugin.Kafka.NewWatcher(subscriber)

	// Prepare subscription channel. Relevant kafka messages are send to this
	// channel so that the watcher can read it.
	plugin.subscription = make(chan messaging.ProtoMessage)
	// The watcher is consuming messages
	err = plugin.kafkaWatcher.Watch(messaging.ToProtoMsgChan(plugin.subscription), topic1)
	if err != nil {
		plugin.Log.Error(err)
	}

	// Prepare subscription channel. Relevant kafka messages are send to this
	// channel so that the watcher can read it
	plugin.asyncSubscription = make(chan messaging.ProtoMessage)
	// The watcher is consuming messages
	err = plugin.kafkaWatcher.Watch(messaging.ToProtoMsgChan(plugin.asyncSubscription), topic2)
	if err != nil {
		plugin.Log.Error(err)
	}

	plugin.Log.Info("Initialization of the custom plugin for the Kafka example is completed")

	// Run sync and async kafka consumers.
	go plugin.syncEventHandler()
	go plugin.asyncEventHandler()

	// Run the producer to send notifications.
	go plugin.producer()

	// Verify results and close the example.
	go plugin.closeExample()

	return err
}

func (plugin *ExamplePlugin) closeExample() {
	for {
		if plugin.messagesSent && plugin.asyncSuccess && plugin.syncRecv && plugin.asyncRecv {
			time.Sleep(2 * time.Second)

			err := plugin.kafkaWatcher.StopWatch(topic1)
			if err != nil {
				plugin.Log.Errorf("Error while stopping watcher: %v", err)
			} else {
				plugin.Log.Info("Sync watcher closed")
			}

			err = plugin.kafkaWatcher.StopWatch(topic2)
			if err != nil {
				plugin.Log.Errorf("Error while stopping watcher: %v", err)
			} else {
				plugin.Log.Info("Async watcher closed")
			}

			plugin.Log.Info("kafka example finished, sending shutdown ...")

			close(plugin.exampleFinished)
			break
		}
	}
}

// Close closes the subscription and the channels used by the async producer.
func (plugin *ExamplePlugin) Close() error {
	return safeclose.Close(plugin.subscription)
}

/***********************
 * Kafka Example calls *
 ***********************/

// Send Kafka notifications
func (plugin *ExamplePlugin) producer() {
	// Wait for the both event handlers to initialize
	time.Sleep(2 * time.Second)

	// Synchronous message with protobuf-encoded data.
	enc := &etcdexample.EtcdExample{
		StringVal: "value",
		Uint32Val: uint32(0),
		BoolVal:   true,
	}
	plugin.Log.Info("Sending Kafka notification (protobuf)")

	// Send several sync messages with offsets offsetLast+1, offsetLast+2,...
	plugin.Log.Infof("Sending %v sync Kafka notifications (protobuf) ...", messageCountNum)
	for i := 0; i < messageCountNum; i++ {
		err := plugin.kafkaSyncPublisher.Put("proto-key", enc)
		if err != nil {
			plugin.Log.Errorf("Failed to sync-send a proto message, error %v", err)
		}
	}

	// Send message with protobuf encoded data asynchronously.
	// Delivery status is propagated back to the application through
	// the configured pair of channels - one for the success events and one for
	// the errors.
	plugin.Log.Infof("Sending %v async Kafka notifications (protobuf) ...", messageCountNum)
	for i := 0; i < messageCountNum; i++ {
		err := plugin.kafkaAsyncPublisher.Put("async-proto-key", enc)
		if err != nil {
			plugin.Log.Errorf("Failed to async-send a proto message, error %v", err)
		}
	}

	// Mark that all messages were sent
	plugin.messagesSent = true
}

/*************
 * Consumers *
 *************/

// syncEventHandler is a Kafka consumer synchronously processing events from
// a channel associated with a specific topic. If a producer sends a message
// matching this destination criteria, the consumer will receive it.
func (plugin *ExamplePlugin) syncEventHandler() {
	plugin.Log.Info("Started Kafka event handler...")
	msgCounter := 0
	if messageCountNum == 0 {
		// Set as done
		plugin.syncRecv = true
	}

	// Watch on message channel for sync kafka events
	for message := range plugin.subscription {
		plugin.Log.Infof("Received Kafka Message, topic '%s', partition '%v', offset '%v', key: '%s', ",
			message.GetTopic(), message.GetPartition(), message.GetOffset(), message.GetKey())
		// mark the offset
		plugin.kafkaWatcher.MarkOffset(message, "")
		msgCounter++
		if msgCounter == messageCountNum {
			plugin.syncRecv = true
		}
	}
}

// asyncEventHandler is a Kafka consumer asynchronously processing events from
// a channel associated with a specific topic. If a producer sends a message
// matching this destination criteria, the consumer will receive it.
func (plugin *ExamplePlugin) asyncEventHandler() {
	plugin.Log.Info("Started Kafka async event handler...")
	msgCounter := 0
	asyncMsgSucc := 0
	if messageCountNum == 0 {
		// Set as done
		plugin.asyncSuccess = true
		plugin.asyncRecv = true
	}
	for {
		select {
		// Channel subscribed with watcher
		case message := <-plugin.asyncSubscription:
			plugin.Log.Infof("Received async Kafka Message, topic '%s', partition '%v', offset '%v', key: '%s', ",
				message.GetTopic(), message.GetPartition(), message.GetOffset(), message.GetKey())
			// mark the offset
			plugin.kafkaWatcher.MarkOffset(message, "")
			msgCounter++
			if msgCounter == messageCountNum {
				plugin.asyncRecv = true
			}
		case message := <-plugin.asyncSuccessChannel:
			plugin.Log.Infof("Async message successfully delivered, topic '%s', partition '%v', offset '%v', key: '%s', ",
				message.GetTopic(), message.GetPartition(), message.GetOffset(), message.GetKey())
			asyncMsgSucc++
			if asyncMsgSucc == messageCountNum {
				plugin.asyncSuccess = true
			}
			// Error callback channel
		case err := <-plugin.asyncErrorChannel:
			plugin.Log.Errorf("Failed to publish async message, %v", err)
		}
	}
}

func resolveMsgCount(count string) (int, error) {
	result, err := strconv.Atoi(count)
	return result, err
}
