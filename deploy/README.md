# TermChat Docker Deployment on Proxmox LXC

This deployment runs three containers in one dedicated LXC:

- `server`: the non-root, read-only TermChat Go server
- `db`: PostgreSQL with a persistent Docker volume
- `migrate`: a one-shot `golang-migrate` container

Nginx remains in the existing reverse-proxy LXC. PostgreSQL has no published host port. Only the TermChat HTTP/WebSocket port is published on the TermChat LXC's LAN address.

## 1. Prepare the Proxmox LXC

Use an unprivileged Debian LXC with a static LAN address. Docker inside an unprivileged Proxmox LXC requires the `nesting` and `keyctl` features. Stop the LXC and enable both in the Proxmox UI under **Options → Features**, then start it again.

Proxmox documents these feature flags at:

- <https://pve.proxmox.com/pve-docs/pct.conf.5.html>

For stronger isolation than nested containers, Proxmox recommends a VM. This guide follows the chosen LXC setup.

## 2. Install Docker Engine in the LXC

These commands target Debian. Run them as root inside the LXC:

```bash
apt update
apt install -y ca-certificates curl openssl git
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg \
  -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

. /etc/os-release
ARCH="$(dpkg --print-architecture)"
printf '%s\n' \
  'Types: deb' \
  'URIs: https://download.docker.com/linux/debian' \
  "Suites: ${VERSION_CODENAME}" \
  'Components: stable' \
  "Architectures: ${ARCH}" \
  'Signed-By: /etc/apt/keyrings/docker.asc' \
  > /etc/apt/sources.list.d/docker.sources

apt update
apt install -y \
  docker-ce \
  docker-ce-cli \
  containerd.io \
  docker-buildx-plugin \
  docker-compose-plugin

systemctl enable --now docker
docker run --rm hello-world
docker compose version
```

Official Debian installation instructions:

- <https://docs.docker.com/engine/install/debian/>

## 3. Put the repository in the LXC

Keep the Docker deployment separate from the earlier native-binary experiment:

```bash
install -d -m 0750 /srv/termchat
```

If the repository has a remote:

```bash
git clone REPOSITORY_URL /srv/termchat
cd /srv/termchat
```

Alternatively, copy a committed source archive from the development machine and extract it into `/srv/termchat`.

## 4. Create production secrets

```bash
cd /srv/termchat/deploy
cp compose.env.example compose.env
chmod 0600 compose.env
openssl rand -hex 32
openssl rand -hex 32
```

Use the first generated value for `POSTGRES_PASSWORD` and the second for `TERMCHAT_JWT_SECRET`. Edit the file:

```bash
nano compose.env
```

Set at least:

```dotenv
TERMCHAT_BIND_IP=TERMCHAT_LXC_STATIC_LAN_IP
TERMCHAT_PORT=8080
POSTGRES_DB=termchat
POSTGRES_USER=termchat
POSTGRES_PASSWORD=FIRST_GENERATED_HEX_VALUE
TERMCHAT_JWT_SECRET=SECOND_GENERATED_HEX_VALUE
TERMCHAT_TOKEN_TTL=24h
TERMCHAT_CLEANUP_INTERVAL=1h
TERMCHAT_IMAGE=termchat-server:local
```

Keep both generated secrets stable across updates. Changing the PostgreSQL password without updating the existing database role breaks database access. Changing the JWT secret invalidates all existing sessions.

Hex values are required here because the Compose file safely embeds the database password in a PostgreSQL URL.

## 5. Validate and start the stack

Run these commands from `/srv/termchat/deploy`:

```bash
docker compose --env-file compose.env -f compose.yml config --quiet
docker compose --env-file compose.env -f compose.yml pull db migrate
docker compose --env-file compose.env -f compose.yml up -d --build --wait
```

Inspect all services, including the completed migration container:

```bash
docker compose --env-file compose.env -f compose.yml ps -a
```

Expected state:

- `db`: `healthy`
- `migrate`: `Exited (0)` — this is normal for a one-shot job
- `server`: `healthy`

Verify the backend from the TermChat LXC:

```bash
curl -fsS http://127.0.0.1:8080/healthz
```

If `TERMCHAT_BIND_IP` is a specific LAN address and loopback does not reach the published port, use that LAN address in the health URL.

Expected response:

```json
{"status":"ok"}
```

View logs without printing environment secrets:

```bash
docker compose --env-file compose.env -f compose.yml logs --tail=100 server
docker compose --env-file compose.env -f compose.yml logs --tail=100 db
```

## 6. Configure the separate Nginx LXC

The example is in `nginx-termchat.conf.example`.

1. Put the `map` block once in Nginx's `http` context, outside all `server` blocks.
2. Put the `location /` block inside the HTTPS `server` block for the TermChat domain.
3. Replace `TERMCHAT_LXC_IP` with the TermChat LXC's static LAN address.
4. Keep the existing certificate directives.

Validate and reload Nginx:

```bash
nginx -t
systemctl reload nginx
```

From the Nginx LXC, verify the direct backend path:

```bash
curl -fsS http://TERMCHAT_LXC_IP:8080/healthz
```

Then verify TLS through the public/private domain:

```bash
curl -fsS https://CHAT_DOMAIN/healthz
```

The terminal client connects with:

```bash
termchat --server https://CHAT_DOMAIN
```

The client automatically converts that URL to `wss://CHAT_DOMAIN/v1/ws` for chat.

## 7. Restrict network access

Allow TCP port `8080` on the TermChat LXC only from the Nginx LXC's IP. PostgreSQL port `5432` is not published by this Compose stack.

Prefer the Proxmox firewall for the LXC boundary. Docker's official documentation warns that published container ports can bypass `ufw`/`firewalld` rules unless Docker-specific firewall handling is used.

## 8. Back up PostgreSQL

Create an external backup directory and run a logical dump:

```bash
cd /srv/termchat/deploy
install -d -m 0700 /srv/termchat-backups
docker compose --env-file compose.env -f compose.yml exec -T db \
  sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > "/srv/termchat-backups/termchat-$(date +%F-%H%M%S).dump"
```

Copy these dumps outside the LXC, for example to a NAS or Proxmox Backup Server. A Docker volume and a Proxmox snapshot are not substitutes for a tested PostgreSQL restore.

## 9. Update the running service

Only deploy clean, tested, committed code. On the development machine:

```bash
go test ./... -count=1
git status --short
```

`git status --short` must be empty. Push the commit only when the LXC obtains releases from a Git remote.

On the TermChat LXC:

```bash
cd /srv/termchat
git pull --ff-only
VERSION="$(git rev-parse --short=12 HEAD)"
```

Back up the database before a release containing migrations. Then edit `deploy/compose.env` and set an immutable image tag:

```dotenv
TERMCHAT_IMAGE=termchat-server:COMMIT_HASH
```

Use the value printed by `printf '%s\n' "$VERSION"` in place of `COMMIT_HASH`.

Build the new image without touching the running server:

```bash
cd /srv/termchat/deploy
docker compose --env-file compose.env -f compose.yml build --pull server
```

Run all pending migrations explicitly. This must succeed before the server is replaced:

```bash
docker compose --env-file compose.env -f compose.yml run --rm migrate
```

Recreate only the server service:

```bash
docker compose --env-file compose.env -f compose.yml \
  up -d --no-deps --wait server
```

Verify:

```bash
docker compose --env-file compose.env -f compose.yml ps -a
curl -fsS http://127.0.0.1:8080/healthz
docker compose --env-file compose.env -f compose.yml logs --tail=100 server
```

A server recreation briefly closes current WebSocket connections. The current architecture must remain at one server replica because room presence and broadcasts are held in process memory.

## 10. Roll back the server image

Keep old `termchat-server:<commit>` images. To roll back an application-only change:

1. Set `TERMCHAT_IMAGE` in `compose.env` to the previous tag.
2. Run:

```bash
docker compose --env-file compose.env -f compose.yml \
  up -d --no-deps --wait server
```

3. Repeat the health and log checks.

Do not automatically run down-migrations during rollback. A destructive schema migration can make an old binary incompatible, so production migrations should follow an expand/deploy/contract sequence.

## 11. Stop or remove the stack

Stop without removing containers:

```bash
docker compose --env-file compose.env -f compose.yml stop
```

Remove containers and networks while preserving PostgreSQL data:

```bash
docker compose --env-file compose.env -f compose.yml down
```

Do not add `-v` unless the explicit intention is to permanently delete the PostgreSQL volume and all TermChat data.

## Public exposure warning

Before exposing TermChat broadly to the internet, add server-side rate limiting for login, registration, and room-password attempts, plus WebSocket heartbeat/reconnect behavior in the client. The current message-send rate limiter does not cover those authentication paths.
