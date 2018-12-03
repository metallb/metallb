# Multiplexer

The multiplexer instance has an access to kafka Brokers. To share the access it allows to create connections.
There are available two connection types one support message of type `[]byte` and the other `proto.Message`.
Both of them allows to create several SyncPublishers and AsyncPublishers that implements `BytesPublisher` interface
or `ProtoPubliser` respectively. The connections also provide API for consuming messages implementing `BytesMessage` 
interface or `ProtoMessage` respectively.


```
   
    +-----------------+                                  +---------------+
    |                 |                                  |               |
    |  Kafka brokers  |        +--------------+     +----| SyncPublisher |
    |                 |        |              |     |    |               |
    +--------^--------+    +---| Connection   <-----+    +---------------+
             |             |   |              |
   +---------+----------+  |   +--------------+
   |  Multiplexer       |  |
   |                    <--+
   | SyncProducer       <--+   +--------------+
   | AsyncProducer      |  |   |              |
   | Consumer           |  |   | Connection   <-----+    +----------------+
   |                    |  +---|              |     |    |                |
   |                    |      +--------------+     +----| AsyncPublisher |
   +--------------------+                                |                | 
                                                         +----------------+

```