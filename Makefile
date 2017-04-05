VERSION = $(shell cat VERSION)

default:
	install

build: xcompile
	npm run build
	cp -r ./static ./build/
	cp ./index.html ./build/

install:
	go install .

xcompile:
	env GOOS=linux GOARCH=amd64 go build -ldflags "-X main.VERSION=$(VERSION)" -o build/godown_linux .
	env GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.VERSION=$(VERSION)" -o build/godown_mac .
	env GOOS=windows GOARCH=386 go build -ldflags "-X main.VERSION=$(VERSION)" -o build/godown_win.exe .
