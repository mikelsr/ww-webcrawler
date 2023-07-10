all: clean wasm

wasm:
	@mkdir -p ./wasm
	@env GOOS=wasip1 GOARCH=wasm gotip build -o ./wasm/crawler.wasm ./crawler/crawler.go ./crawler/ww.go

clean:
	@rm -rf ./wasm
