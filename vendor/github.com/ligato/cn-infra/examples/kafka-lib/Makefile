
build:
	cd asyncproducer && go build -i
	cd consumer && go build -i
	cd syncproducer && go build -i
	cd mux && go build -i

clean:
	rm -f asyncproducer/asyncproducer
	rm -f consumer/consumer
	rm -f syncproducer/syncproducer
	rm -f mux/mux