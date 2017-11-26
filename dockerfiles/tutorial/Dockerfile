FROM ubuntu:latest
RUN apt update && apt -y install golang
COPY ./bird_spy.go /
RUN go build -o /bird_spy /bird_spy.go

FROM ubuntu:latest

RUN apt update && apt -y install bird iproute2 iptables netcat
COPY ./run_bird.sh /
COPY --from=0 /bird_spy /
ENTRYPOINT ["/run_bird.sh"]
