# generate go structures from proto files
define generate_sources
	$(if $(shell command -v protoc --gogo_out=. 2> /dev/null),$(info gogo/protobuf is installed),$(error gogo/protobuf missing, please install it with go get github.com/gogo/protobuf))
	@echo "# generating sources"
	@cd view && go generate -v
endef

all: generate build

generate:
	$(call generate_sources)
	@echo "# done"

clean:
	rm -f editor/editor
	rm -f view/view
	rm -f watcher/watcher

build:
	cd view && go build -i
	cd editor && go build -i
	cd watcher && go build -i
