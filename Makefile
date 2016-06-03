default:
	install

build: xcompile
	npm build
	cp -r ./static ./build/
	cp ./index.html ./build/

install:
	go install .

xcompile:
	env GOOS=linux GOARCH=amd64 go build -o build/godown_linux .
	env GOOS=darwin GOARCH=amd64 go build -o build/godown_mac .
	env GOOS=windows GOARCH=386 go build -o build/godown_win .
