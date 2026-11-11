#!/usr/bin/env bash
# Установка dualsense-tray: бинарь, иконки, user unit.
set -euo pipefail
cd "$(dirname "$0")"
command -v go >/dev/null || { echo "нужен Go 1.21+"; exit 1; }

echo "==> сборка"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dualsense-tray .
install -Dm755 dualsense-tray "$HOME/.local/bin/dualsense-tray"

echo "==> иконки"
IC="$HOME/.local/share/icons/hicolor/scalable/apps"
install -Dm644 icons/*.svg -t "$IC"

echo "==> systemd"
install -Dm644 systemd/dualsense-tray.service "$HOME/.config/systemd/user/dualsense-tray.service"
systemctl --user daemon-reload
systemctl --user enable --now dualsense-tray.service

echo "==> готово"
echo "Требуется dualsense-autoswitch (команда dualsense-toggle)."
