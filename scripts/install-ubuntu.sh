#!/usr/bin/env bash
# Instala dependências no Ubuntu e o comando `iconia` em ~/.local/bin.
set -euo pipefail
ROOT="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")/.." && pwd)"

# shellcheck source=/dev/null
. /etc/os-release
echo "== $PRETTY_NAME"

# android-sdk-platform-tools-common traz as regras udev que liberam o adb
# para tablets Acer (vendor 0502) sem sudo.
sudo apt-get update
sudo apt-get install -y x11vnc x11-xserver-utils xserver-xorg-core \
	adb android-sdk-platform-tools-common curl make
sudo udevadm control --reload-rules && sudo udevadm trigger || true

# Go do apt só é usado se for >= 1.22 (Ubuntu 24.04+); senão o relay
# é baixado da release na primeira execução.
goc="$(apt-cache policy golang-go)"
if grep -qE 'Candidate: 2:1\.(2[2-9]|[3-9][0-9])' <<<"$goc"; then
	sudo apt-get install -y golang-go
	make -C "$ROOT" relay
fi

make -C "$ROOT" install
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) echo "Adicione ~/.local/bin ao PATH (ou abra um novo terminal)." ;; esac

if [[ "$(printf '%s\n' "$VERSION_ID" 25.10 | sort -V | head -n1)" == 25.10 ]]; then
	echo "ATENÇÃO: Ubuntu $VERSION_ID não tem sessão GNOME em Xorg. Veja 'Ubuntu 25.10+' no README."
elif [[ "${XDG_SESSION_TYPE:-}" != x11 ]]; then
	echo "Próximo passo: saia da sessão e entre em 'Ubuntu on Xorg' (engrenagem na tela de login)."
fi
echo "Pronto. Conecte o tablet e rode: iconia doctor"
