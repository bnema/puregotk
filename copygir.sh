#!/bin/sh

set -e

podman build . -t puregotk-gir-fetcher -f gir.Dockerfile
podman create --name puregotk-gir-fetcher puregotk-gir-fetcher

GIR_DIR="internal/gir/spec/"
find "${GIR_DIR}"*.gir -type f | while read filename; do
  podman cp "puregotk-gir-fetcher:/gir-files/$(basename ${filename})" "${GIR_DIR}" || true
done
podman rm puregotk-gir-fetcher
