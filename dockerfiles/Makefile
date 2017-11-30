all: dockerfiles

.PHONY: dockerfiles dev-images

dockerfiles: prod/controller prod/bgp-speaker dev/controller dev/bgp-speaker

prod/%: prod.tmpl
	sed -e "s/%%BINARY%%/$*/g" $< >$@

dev/%: dev.tmpl
	sed -e "s/%%BINARY%%/$*/g" $< >$@
