#!/bin/sh 

cd `dirname $0`

mkdir -p pb

protoc --proto_path=./protobuf --go_out=:./pb --go-grpc_out=:./pb ./protobuf/test.proto

echo "build to `pwd`/pb"