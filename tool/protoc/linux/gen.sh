#!/bin/bash
./protoc  --plugin=./protoc-gen-go  --go_out=.  *.proto
