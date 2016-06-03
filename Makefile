default:

install:
	go install ./...

xcompile:
	env GOOS=linux GOARCH=amd64 go build -o bin/godown_linux .
	env GOOS=darwin GOARCH=amd64 go build -o bin/godown_mac .
	env GOOS=windows GOARCH=386 go build -o bin/godown_win .
