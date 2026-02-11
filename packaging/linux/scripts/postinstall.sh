#!/bin/bash
set -e

# Set ownership of data directories
chown -R fedaaa:fedaaa /var/lib/fedaaa || true
chown -R fedaaa:fedaaa /var/log/fedaaa || true

# Reload systemd daemon
if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true

    # Enable service but don't start it yet
    systemctl enable fedaaa.service || true

    echo ""
    echo "FedAAA has been installed successfully!"
    echo ""
    echo "Next steps:"
    echo "1. Edit the configuration: /etc/fedaaa/config.yaml"
    echo "2. Start the service: sudo systemctl start fedaaa"
    echo "3. Check status: sudo systemctl status fedaaa"
    echo "4. View logs: sudo journalctl -u fedaaa -f"
    echo ""
fi

exit 0
