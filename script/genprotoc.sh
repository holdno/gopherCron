#!/bin/sh 

cd `dirname $0`
cd ../

mkdir -p pkg/cronpb

protoc --proto_path=./etc/protobuf --go_out=:./pkg/cronpb --go-grpc_out=:./pkg/cronpb gophercron.proto

echo "build to `pwd`/pkg/cronpb"