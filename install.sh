#!/usr/bin/env bash
# ============================================================
# GoBackup – Build & Install Script für Debian/Ubuntu
# ============================================================
set -euo pipefail

BINARY_NAME="backup-tool"
INSTALL_DIR="/usr/local/bin"
DESKTOP_DIR="/usr/share/applications"
ICON_DIR="/usr/share/icons/hicolor/256x256/apps"

# ── Check dependencies ────────────────────────────────────────────────────────
check_dep() {
    if ! command -v "$1" &>/dev/null; then
        echo "❌ '$1' nicht gefunden. Installiere: sudo apt install $2"
        exit 1
    fi
}

echo "🔍 Prüfe Abhängigkeiten…"
check_dep go "golang-go"
check_dep gcc "build-essential"
check_dep lsblk "util-linux"

# ── Build ─────────────────────────────────────────────────────────────────────
echo ""
echo "📦 Installiere Go-Abhängigkeiten…"
go mod download

echo ""
echo "🔨 Kompiliere GoBackup (CGO_ENABLED=1 für Fyne)…"
CGO_ENABLED=1 go build -ldflags="-s -w" -o "$BINARY_NAME" .

echo "✅ Build erfolgreich: ./$BINARY_NAME"

# ── Install (optional, needs root) ───────────────────────────────────────────
if [[ "${1:-}" == "--install" ]]; then
    if [[ $EUID -ne 0 ]]; then
        echo "❌ --install benötigt Root-Rechte (sudo ./install.sh --install)"
        exit 1
    fi

    echo ""
    echo "📂 Installiere nach $INSTALL_DIR …"
    install -m 755 "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

    echo "🖥️  Erstelle .desktop Eintrag…"
    cat > "$DESKTOP_DIR/gobackup.desktop" <<EOF
[Desktop Entry]
Name=GoBackup
Comment=Block-Level Disk Backup & Restore
Exec=sudo $INSTALL_DIR/$BINARY_NAME
Icon=gobackup
Terminal=false
Type=Application
Categories=System;Utility;
Keywords=backup;disk;restore;
EOF
    chmod 644 "$DESKTOP_DIR/gobackup.desktop"

    echo ""
    echo "✅ Installation abgeschlossen!"
    echo "   Starte die App mit: sudo $INSTALL_DIR/$BINARY_NAME"
fi

# ── Print usage ───────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Verwendung:"
echo "   sudo ./$BINARY_NAME           Startet die GUI"
echo "   sudo ./install.sh --install   Systemweit installieren"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
