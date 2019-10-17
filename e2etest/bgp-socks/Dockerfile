FROM debian:stable-slim

RUN apt update
RUN apt -y --no-install-recommends install bird dante-server supervisor iproute2
ADD entrypoint.sh /

CMD ["/entrypoint.sh"]
