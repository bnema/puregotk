#!/usr/bin/env bash

set -e

echo "generating go files..."
go generate

echo "running go vet..."
go vet -unsafeptr=false -stdmethods=false ./v4/...
