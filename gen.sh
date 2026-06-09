#!/usr/bin/env bash

set -e

echo "generating go files..."
go generate

echo "running go vet..."
# TODO(puregotk maintainers, before v1.0): re-enable unsafeptr once generated
# pointer-cast wrappers are audited or covered by targeted tests; current GIR
# output intentionally round-trips C pointers through uintptr and trips vet
# false positives across many generated files.
go vet -unsafeptr=false ./v4/...
