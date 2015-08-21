# statically link go 
SHELL=/bin/bash
GOPATH=${HOME}/container/go:${PWD}
CGO_ENABLED=0

targets:=cfggen

all: ${targets}
	@echo targets=${targets}

% : %.go *.go
	CGO_ENABLED=0 GOPATH=${GOPATH} go build -a -ldflags '-s' -o $@ $^

test: test-cfggen

PASSTHROUGH=false
test-cfggen : cfggen
ifeq ($(PASSTHROUGH),true)	
	rm -f formatted.config.json unformatted.config.json
	@echo
	@echo target ./cfggen --format=false --passthrough --config-file config.json --config-template node.yaml --dump --dump-tmpl node.yaml.txt

	@echo
	-@ ./cfggen --format=false --passthrough --config-file config.json --config-template node.yaml -dump --dump-tmpl node.yaml.txt
	@echo
	@./cfggen --format --passthrough --config-file config.json --config-template node.yaml --output formatted.config.json
endif
	@./cfggen --config-file config.json --config-template node.yaml

test : ${targets}
	echo $@ $^ $<
	for file in ${targets}; do ./$${file}; done

clean:
	rm -f ${targets}

include github.fork.Makefile
