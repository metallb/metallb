all: dockerfiles

.PHONY: dockerfiles dev-images

dockerfiles: prod/controller prod/bgp-speaker

prod/%: prod.tmpl
	sed -e "s/%%BINARY%%/$*/g" $< >$@

dev-images:
	rm -rf staging
	mkdir -p staging
	go install ../controller ../bgp-speaker
	go build -o staging/controller ../controller
	go build -o staging/bgp-speaker ../bgp-speaker
	sed -e 's/%%BINARY%%/controller/g' dev.tmpl >staging/Dockerfile
	cd staging && sudo docker build -t metallb/controller:dev .
	sed -e 's/%%BINARY%%/bgp-speaker/g' dev.tmpl >staging/Dockerfile
	cd staging && sudo docker build -t metallb/bgp-speaker:dev .
	rm -rf staging
