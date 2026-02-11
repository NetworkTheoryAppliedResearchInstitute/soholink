#!/bin/bash
set -e

# Stop and disable the service before removal
if command -v systemctl >/dev/null 2>&1; then
    systemctl stop fedaaa.service || true
    systemctl disable fedaaa.service || true
fi

exit 0
