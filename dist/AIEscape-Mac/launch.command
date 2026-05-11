#!/bin/bash
# Detect whether we're running on Apple Silicon or Intel and pick the right binary.
cd "$(dirname "$0")"
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    ./ai-escape-arm64
else
    ./ai-escape
fi
