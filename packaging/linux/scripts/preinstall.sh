#!/bin/bash
set -e

# Create fedaaa user and group if they don't exist
if ! getent group fedaaa > /dev/null 2>&1; then
    groupadd --system fedaaa
fi

if ! getent passwd fedaaa > /dev/null 2>&1; then
    useradd --system --gid fedaaa --home-dir /var/lib/fedaaa \
        --no-create-home --shell /usr/sbin/nologin \
        --comment "FedAAA service user" fedaaa
fi

# Add fedaaa user to docker group if it exists
if getent group docker > /dev/null 2>&1; then
    usermod -aG docker fedaaa || true
fi

exit 0
