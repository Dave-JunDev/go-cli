#!/usr/bin/env bash
set -euo pipefail

BINARY="k8s-ui"
INSTALL_DIR="${HOME}/.local/bin"

echo "Building ${BINARY}..."
go build -ldflags="-s -w" -o "${BINARY}" ./main.go
echo "Done: ./${BINARY} ($(du -h "${BINARY}" | cut -f1))"

if [ "${1:-}" = "install" ]; then
	mkdir -p "${INSTALL_DIR}"
	mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
	echo "Installed to ${INSTALL_DIR}/${BINARY}"
	echo "Make sure ${INSTALL_DIR} is in your PATH"
fi
