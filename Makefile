ifeq ($(origin GOPATH), undefined)
    GOPATH := $(HOME)/go
endif

http_dir := services/http
http_proto := ${http_dir}/proto
http_proto_pkg := ${http_proto}/pkg

.PHONY: all clean services wasm

all: clean services wasm

services:
	@mkdir ${http_proto_pkg}
	@capnp compile -I$(GOPATH)/src/capnproto.org/go/capnp/v3/std -ogo:${http_proto_pkg} --src-prefix=${http_proto} ${http_proto}/http.capnp

wasm:
	@mkdir -p ./wasm
	@env GOOS=wasip1 GOARCH=wasm gotip build -o ./wasm/crawler.wasm ./crawler/crawler.go ./crawler/http.go ./crawler/neo4j.go

clean:
	@rm -rf ./wasm ${http_proto_pkg}
