#!/bin/sh

# snap install protobuf --classic
#GIT_TAG="v1.2.0" # change as needed
# go get -d -u github.com/golang/protobuf/protoc-gen-go
# git -C "$(go env GOPATH)"/src/github.com/golang/protobuf checkout $GIT_TAG
# go install github.com/golang/protobuf/protoc-gen-go
/snap/bin/protoc --go_out=plugins=grpc:. *.proto
