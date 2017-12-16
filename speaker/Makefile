include ../Makefile.inc

NAME:=speaker
BINARY:=$(NAME)
DOCKER_IMAGE_NAME:=$(DOCKER)/$(NAME)

.PHONY: speaker
speaker:
	CGO_ENABLED=0 $(SYSTEM) go build -o $(BINARY)
