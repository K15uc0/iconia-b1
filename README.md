# iconia-b1

Usa um tablet Android antigo (Acer Iconia B1 e similares, Android 4.1+) como **monitor auxiliar** de um PC Linux, por **USB** ou **Wi‑Fi**, com toque funcionando como mouse.

Não exige root no tablet nem driver de vídeo especial no PC.

## Como funciona

```
 PC Linux                                                   Tablet (Android 4.x)
┌──────────────────────────────────────────────┐           ┌──────────────────────────┐
│ monitor virtual (xrandr / sway / KWin)       │           │                          │
│   └─ área 1024x600+1920+0                    │           │  cliente VNC             │
│        └─ x11vnc -clip  :5900 (localhost)    │           │   └─ 127.0.0.1:5900      │
│              ▲                               │           │          │               │
│        iconia-relay host ── adb forward ──── USB ──────► iconia-relay device         │
└──────────────────────────────────────────────┘           └──────────────────────────┘
```

1. **Monitor virtual**: o PC passa a ter uma tela extra do tamanho do tablet. Janelas são arrastadas para ela como em qualquer segundo monitor.
2. **VNC**: só a área desse monitor é exportada (`x11vnc -clip`, `wayvnc -o`, `krfb-virtualmonitor`).
3. **Transporte**:
   - **USB, Android 5+**: `adb reverse`.
   - **USB, Android 4.x**: não há `adb reverse`. O `iconia-relay` (Go, binário estático, sem NDK) roda no tablet e cria um túnel reverso usando só `adb forward`: o PC mantém conexões ociosas abertas até o tablet; quando o cliente VNC conecta, o tablet sinaliza e o PC liga a conexão ao VNC local.
   - **Wi‑Fi**: o tablet conecta direto no IP do PC, com senha VNC.

O script mantém a tela do tablet ligada enquanto estiver no USB (`svc power stayon usb`) e desfaz tudo ao encerrar.

## Suporte

| Sessão no PC | Monitor virtual | Servidor VNC | Situação |
|---|---|---|---|
| X11 (qualquer DE) | saída livre via `xrandr --addmode`, ou framebuffer estendido + `--setmonitor` | `x11vnc` | testado (driver dummy do Xorg) |
| Sway | `swaymsg create_output` | `wayvnc` | implementado, não testado |
| KDE Plasma Wayland | `krfb-virtualmonitor` | embutido | implementado, não testado |
| GNOME Wayland | — | — | não suportado: use a sessão "GNOME on Xorg" |

Tablets: o relay é compilado para `arm`, `arm64`, `386` e `amd64` e escolhido pelo ABI do aparelho (o Iconia One B1‑750, por exemplo, é x86).

Windows e macOS não estão cobertos por este projeto.

## Instalação

### PC

Debian/Ubuntu (X11):

```bash
sudo apt install x11vnc x11-xserver-utils adb
```

Arch: `x11vnc xorg-xrandr android-tools`. Fedora: `x11vnc xrandr android-tools`. Sway: `wayvnc jq`. KDE: `krfb`.

`cvt` (pacote `xserver-xorg-core` no Debian) é usado pelo método de saída virtual; sem ele o script cai no método de framebuffer.

```bash
git clone https://github.com/K15uc0/iconia-b1
cd iconia-b1
make           # compila o relay (requer Go 1.22+); ou baixe build/ da página de releases
make install   # link em ~/.local/bin/iconia
```

### Tablet

1. **Configurações → Sobre o tablet** → toque 7 vezes em **Número da versão**.
2. **Opções do desenvolvedor → Depuração USB**: ativar.
3. Conecte no PC e aceite a chave RSA (Android 4.2.2+ pergunta; 4.1 não).
4. Instale um cliente VNC que ainda rode em Android 4.x, por exemplo versões antigas do **bVNC** ou do **androidVNC**. Confira o `minSdkVersion` do APK antes (Android 4.1 = API 16, 4.2 = 17, 4.4 = 19); não validei versões específicas.

   ```bash
   adb install cliente-vnc.apk
   ```

## Uso

```bash
iconia doctor              # checa dependências, saídas de vídeo e o tablet
iconia start               # USB (padrão), 1024x600 à direita do monitor principal
```

No cliente VNC do tablet, conecte em `127.0.0.1:5900`, sem senha. Ctrl+C no PC encerra e restaura tudo.

Outras opções:

```bash
iconia start --res 1280x800 --pos left     # Iconia B1-730HD, à esquerda
iconia start --method fb                   # força o framebuffer estendido
iconia start --launch                      # abre vnc:// no tablet sozinho (se o app aceitar)
iconia passwd && iconia start --wifi       # Wi-Fi com senha
iconia stop                                # limpa restos se o processo morreu
```

Configuração persistente em `~/.config/iconia-b1/config` (variáveis do topo de [`bin/iconia`](bin/iconia)):

```bash
RES=1024x600
POS=right
LAUNCH_CLIENT=1
```

### Resolução por modelo

| Modelo | Tela |
|---|---|
| B1‑A71, B1‑710, B1‑711 | 1024x600 |
| B1‑730HD, B1‑750 | 1280x800 |

Se o seu não estiver na tabela, `iconia doctor` mostra a resolução real.

## Ajustes no cliente VNC

- Codificação **Tight** ou **ZRLE**; cor de 16 ou 24 bits.
- Modo de entrada **toque direto** para o dedo virar o ponteiro.
- Tela cheia e sem escala: a resolução do monitor virtual já é a nativa.

USB costuma ter latência menor e mais banda que o Wi‑Fi 2.4 GHz desses tablets, além de carregar a bateria.

## Problemas comuns

| Sintoma | Causa / solução |
|---|---|
| `nenhuma saída de vídeo livre` | Driver sem saída desconectada; o modo `auto` usa o framebuffer. Na NVIDIA proprietária, use `--method fb`. |
| `framebuffer … excede o máximo` | Limite do driver; reduza a resolução ou use `--pos below`. |
| Monitor fb some sozinho | Alguns ambientes refazem o layout em hotplug; rode `iconia stop && iconia start`. |
| Cliente conecta e cai na hora | O relay no tablet não subiu: veja a saída do `iconia start` e `adb devices`. |
| `adb devices` mostra `unauthorized` | Aceite a chave no tablet; em Android 4.1, atualize o adb ou use outro cabo/porta. |
| Tela do tablet apaga | Só fica ligada via USB; no Wi‑Fi aumente o tempo de tela em Configurações. |

## Desenvolvimento

```bash
make test    # go vet + testes do relay (inclui teste ponta a ponta)
make lint    # shellcheck
```

Estrutura:

- `bin/iconia`: orquestração (monitor virtual, VNC, adb, limpeza).
- `relay/`: túnel reverso sobre `adb forward` para Android < 5.
