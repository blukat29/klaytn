#!/bin/sh

set -e

for i in $(seq 1 300); do
    echo $i
    go test -run=TestDiffLayerExternalInvalidationPartialFlatten
done
