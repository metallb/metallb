SUBDIRS := controller speaker

all: $(SUBDIRS)
build: $(SUBDIRS)

$(SUBDIRS):
	@ $(MAKE) $(SUBMAKEOPTS) -C $@ all

clean:
	-$(QUIET) for i in $(SUBDIRS); do $(MAKE) $(SUBMAKEOPTS) -C $$i clean; done

.PHONY: all build $(SUBDIRS) clean
