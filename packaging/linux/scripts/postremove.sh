#!/bin/bash
set -e

# Reload systemd after removal
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
fi

# Note: We intentionally do NOT remove the fedaaa user, group, or data directories
# to preserve user data across reinstalls. Users can manually clean up if desired.

echo ""
echo "FedAAA has been removed."
echo ""
echo "Note: User data in /var/lib/fedaaa has been preserved."
echo "To completely remove all data, run:"
echo "  sudo rm -rf /var/lib/fedaaa /var/log/fedaaa /etc/fedaaa"
echo "  sudo userdel fedaaa"
echo "  sudo groupdel fedaaa"
echo ""

exit 0
