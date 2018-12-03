
## Running Kafka on Local Host for ARM64 platform

There is no official spotify/kafka image for ARM64 platform.
You can build an image following steps at the [repository](https://github.com/spotify/docker-kafka#build-from-source).
However you need to modify the kafka/Dockerfile before building like this:
```
#FROM java:openjdk-8-jre
#arm version needs this....
FROM openjdk:8-jre
...
...
#ENV KAFKA_VERSION 0.10.1.0
#arm version needs this....
ENV KAFKA_VERSION 0.10.2.1
...
...

```

1. For preparing kafka-arm64 image please follow these steps:
```
git clone https://github.com/spotify/docker-kafka.git
cd docker-kafka
sed -i -e "s/FROM java:openjdk-8-jre/FROM openjdk:8-jre/g" kafka/Dockerfile
sed -i -e "s/ENV KAFKA_VERSION 0.10.1.0/ENV KAFKA_VERSION 0.10.2.1/g" kafka/Dockerfile
docker build -t kafka-arm64 kafka/
```





2. Then you can start Kafka in a separate container:
```
sudo docker run -p 2181:2181 -p 9092:9092 --name kafka --rm \
 --env ADVERTISED_HOST=172.17.0.1 --env ADVERTISED_PORT=9092 kafka-arm64
```
