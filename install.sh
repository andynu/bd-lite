#!/bin/sh
set -e

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

go build -o bd .
mkdir -p "$INSTALL_DIR"
mv bd "$INSTALL_DIR/bd"

echo "installed bd to $INSTALL_DIR/bd"
