test:
	GOMAXPROCS=1 go test
	GOMAXPROCS=2 go test
	GOMAXPROCS=4 go test
	GOMAXPROCS=8 go test

test-386:
	GOARCH=386 GOMAXPROCS=1 go test
	GOARCH=386 GOMAXPROCS=2 go test
	GOARCH=386 GOMAXPROCS=4 go test
	GOARCH=386 GOMAXPROCS=8 go test

bench-1-goprocs:
	GOMAXPROCS=1 go test -test.bench=".*"

bench-2-goprocs:
	GOMAXPROCS=2 go test -test.bench=".*"

bench-4-goprocs:
	GOMAXPROCS=4 go test -test.bench=".*"

bench-8-goprocs:
	GOMAXPROCS=8 go test -test.bench=".*"

build:
	/snap/bin/protoc --go_out=plugins=grpc:. *.proto
	cd cmd && go build -ldflags="-s -w"
	mv cmd/cmd server

deploy: build
	scp server root@dev.subiz.net:/opt/gorpc
	scp gorpc.json root@dev.subiz.net:/etc/gorpc.json
	ssh -t -t root@dev.subiz.net  /opt/run.sh
