# TermChat

TermChat is a Windows-, Linux-, and Apple Silicon macOS-compatible terminal chat application built around private rooms. Users can clone this repository, build only the client, and connect to an existing TermChat server; Docker and PostgreSQL are not required to use the client.

## Requirements

- [Git](https://git-scm.com/downloads)
- [Go 1.27 or later](https://go.dev/dl/)
- An internet connection and a modern terminal

Verify your Go installation before continuing:

```text
go version
```

## Windows

Open PowerShell and run these commands in order:

```powershell
git clone https://github.com/xdjanisxd/termchat.git
cd termchat

New-Item -ItemType Directory -Force ./bin | Out-Null
go build -o ./bin/termchat.exe ./cmd/client

curl.exe -fsS "https://termchat.osmanela.xyz/healthz"
./bin/termchat.exe --server "https://termchat.osmanela.xyz"
```

Expected health check response:

```json
{"status":"ok"}
```

To set the server address for the current PowerShell session:

```powershell
$env:TERMCHAT_SERVER_URL = "https://termchat.osmanela.xyz"
./bin/termchat.exe
```

## Linux

Open a terminal and run these commands in order:

```bash
git clone https://github.com/xdjanisxd/termchat.git
cd termchat

mkdir -p ./bin
CGO_ENABLED=0 go build -o ./bin/termchat ./cmd/client
chmod +x ./bin/termchat

curl -fsS "https://termchat.osmanela.xyz/healthz"
./bin/termchat --server "https://termchat.osmanela.xyz"
```

Expected health check response:

```json
{"status":"ok"}
```

To set the server address for the current shell session:

```bash
export TERMCHAT_SERVER_URL="https://termchat.osmanela.xyz"
./bin/termchat
```

## macOS (Apple Silicon)

This build supports Apple Silicon Macs (M1 or newer). Intel Macs are not currently targeted.

Open Terminal and run these commands in order:

```bash
git clone https://github.com/xdjanisxd/termchat.git
cd termchat

./scripts/build-macos-arm64-client.sh
chmod +x ./dist/termchat-darwin-arm64

curl -fsS "https://termchat.osmanela.xyz/healthz"
./dist/termchat-darwin-arm64 --server "https://termchat.osmanela.xyz"
```

Expected health check response:

```json
{"status":"ok"}
```

To set the server address for the current shell session:

```bash
export TERMCHAT_SERVER_URL="https://termchat.osmanela.xyz"
./dist/termchat-darwin-arm64
```

## First Use

When the client opens, register a new user or sign in with an existing account. Basic commands:

| Command | Description |
|---|---|
| `/createroom <room-name> <password>` | Creates a private room |
| `/join <room-name>` | Joins a room through a masked password prompt |
| `/who` | Shows users currently online in the room |
| `/l` | Leaves the room |
| `/help` | Opens the scrollable command reference; press `Esc` to close it |
| `/q` | Closes the application |

While chatting, use `PgUp` and `PgDn` to scroll through message history; the composer and status bar remain visible. The help screen uses the same keys when its command list is taller than the terminal.

Do not append `/v1` or `/v1/ws` to the server URL; the client creates the required API and WebSocket paths automatically.

## Troubleshooting

If the server cannot be reached, check this endpoint first:

```text
https://termchat.osmanela.xyz/healthz
```

To clear a stale or invalid session:

**Windows PowerShell**

```powershell
Remove-Item "$env:APPDATA\termchat\session.json" -Force -ErrorAction SilentlyContinue
```

**Linux**

```bash
rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/termchat/session.json"
```

**macOS**

```bash
rm -f "$HOME/Library/Application Support/termchat/session.json"
```

To run the server in your own environment, see the [Docker deployment guide](deploy/README.md).
