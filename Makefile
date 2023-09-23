ifeq ($(origin GOPATH), undefined)
    GOPATH := $(HOME)/go
endif

http_dir := services/http
http_proto := ${http_dir}/proto
http_proto_pkg := ${http_proto}/pkg
reg_servs := ./services/register_services

crawler_proto := crawler/proto
crawler_proto_pkg := ${crawler_proto}/pkg

.PHONY: all crawler_capnp clean services wasm

all: clean services crawler_capnp wasm

services:
	@mkdir -p ${http_proto_pkg}
	@capnp compile -I$(GOPATH)/src/capnproto.org/go/capnp/v3/std \
		-ogo:${http_proto_pkg} \
		--src-prefix=${http_proto} \
		${http_proto}/http.capnp
	@go build -o ${reg_servs} ./services

crawler_capnp:
	@mkdir -p ${crawler_proto_pkg}
	@capnp compile -I$(GOPATH)/src/capnproto.org/go/capnp/v3/std \
		-ogo:${crawler_proto_pkg} \
		--src-prefix=${crawler_proto} \
		${crawler_proto}/crawler.capnp

wasm:
	@mkdir -p ./wasm
	@env GOOS=wasip1 GOARCH=wasm go build -o \
		./wasm/crawler.wasm \
		./crawler/main.go \
		./crawler/crawler.go \
		./crawler/msg.go \
		./crawler/set.go \
		./crawler/uniquequeue.go \
		./crawler/timedset.go \
		./crawler/http.go \
		./crawler/neo4j.go

clean:
	@rm -rf ./wasm ${http_proto_pkg} ${reg_servs} ${crawler_proto_pkg}
