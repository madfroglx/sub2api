# Sub2API Project Notes

## Local Development

Use the helper script through Makefile:

```bash
make dev
make dev-restart
make dev-stop
make dev-status
make dev-logs
```

`make dev` starts both services:

- Backend: `go run ./cmd/server`, default `http://localhost:8080`
- Frontend: Vite dev server, default `http://localhost:3000`

Frontend edits are applied by Vite HMR and normally do not need a restart.
Backend Go changes need:

```bash
make dev-restart
```

Local runtime state is under `.dev/`.

## Deploy To sk

Use:

```bash
make deploy-sk
```

This runs `scripts/deploy-sk.sh`.

Deployment behavior:

- Creates a local source archive.
- Excludes `.git`, `.dev`, local configs, `deploy/.env`, dependency directories, build outputs, and macOS AppleDouble files.
- Uploads the archive to `sk:/tmp`.
- Extracts source to `/opt/sub2api-src/releases/<build-id>`.
- Updates `/opt/sub2api-src/current`.
- Creates a temporary remote swap file for build stability when the server has no swap, then removes it on exit.
- Builds frontend on `sk` with `npm exec --package pnpm@10.15.1`.
- Builds backend on `sk` with Go `1.26.4` and `-tags embed`.
- Writes build output to `/opt/sub2api-src/build/sub2api.<build-id>` and `/opt/sub2api-src/build/sub2api.clean`.
- Backs up current runtime binary to `/opt/sub2api/sub2api.backup.<build-id>`.
- Replaces `/opt/sub2api/sub2api`.
- Restarts `sub2api` systemd service.
- Rolls back automatically if the new service fails to start.
- Verifies `http://106.54.56.30:8000/` and `/api/v1/auth/me`.

Useful overrides:

```bash
SUB2API_DEPLOY_HOST=sk make deploy-sk
SUB2API_BUILD_ID=manual-$(date +%Y%m%d%H%M%S) make deploy-sk
SUB2API_GO_VERSION=1.26.4 SUB2API_PNPM_VERSION=10.15.1 make deploy-sk
SUB2API_PUBLIC_URL=http://106.54.56.30:8000 make deploy-sk
SUB2API_BUILD_SWAP_SIZE=0 make deploy-sk
```

Do not upload or overwrite production config files during normal code deploys.
Runtime config remains on the server:

- `/etc/sub2api/sub2api.env`
- `/opt/sub2api/config.yaml`

## Server Runtime

Current non-Docker runtime:

- Service: `sub2api`
- Runtime binary: `/opt/sub2api/sub2api`
- Source/build root: `/opt/sub2api-src`
- Port: `8000`
- Public URL: `http://106.54.56.30:8000`

Useful checks:

```bash
ssh sk 'systemctl is-active sub2api'
ssh sk 'sudo journalctl -u sub2api -n 100 --no-pager'
ssh sk 'sudo ss -lntp | grep :8000'
```
