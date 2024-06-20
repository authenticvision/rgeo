#!/usr/bin/env bash
set -eu

TAG=v5.1.2

echo "Updating dataset..."
if ! [ -d natural-earth-vector ]; then
	git clone https://github.com/nvkelso/natural-earth-vector.git --branch "$TAG" --depth 1
else
	pushd natural-earth-vector
	git fetch
	git checkout "$TAG"
	popd
fi

make clean
make -j$(nproc) geodata
