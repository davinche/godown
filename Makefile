VERSION = $(shell cat VERSION)
GOVERSION = $(shell go version | cut -d ' ' -f3)

default:
	install

build: clean xcompile
	npm run build
	cp -r ./static ./build/
	cp ./index.html ./build/
	tar -czf "$(GOVERSION)-godownv$(VERSION).tar.gz" ./build/*

install:
	go install .

xcompile:
	env GOOS=linux GOARCH=amd64 go build -ldflags "-X main.VERSION=$(VERSION)" -o build/godown_linux
	env GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.VERSION=$(VERSION)" -o build/godown_mac
	env GOOS=windows GOARCH=386 go build -ldflags "-X main.VERSION=$(VERSION)" -o build/godown_win.exe
clean:
	rm -rf ./build
