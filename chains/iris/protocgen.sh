#!/usr/bin/env bash

set -eo pipefail
 
proto_dirs=$(find chains/iris/proto -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  buf protoc \
  -I "chains/iris/proto" \
  -I "chains/iris/third_party/proto" \
  --gogofaster_out=\
Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/duration.proto=github.com/golang/protobuf/ptypes/duration,\
plugins=grpc,paths=source_relative:. \
  $(find "${dir}" -maxdepth 1 -name '*.proto')
done

cp -r ./tendermint/* ./chains/iris/proto/*
rm -rf tendermint