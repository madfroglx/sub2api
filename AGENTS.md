# Sub2API Project Notes

## Local Development

### macOS / Linux / Git Bash

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

Equivalent direct script commands:

```bash
./scripts/dev.sh start
./scripts/dev.sh restart backend
./scripts/dev.sh stop
./scripts/dev.sh status
./scripts/dev.sh logs
```

### Windows PowerShell

If `make` is available, the same Makefile commands above can be used. If `make` is not available, prefer running the services with PowerShell-compatible commands.

Start backend from source:

```powershell
$env:DATA_DIR = "$PWD\.dev\backend-data"
$env:SERVER_HOST = "127.0.0.1"
$env:SERVER_PORT = "8080"
$env:SERVER_MODE = "debug"
& "$PWD\.dev\tools\go\bin\go.exe" run ./cmd/server
```

Start frontend:

```powershell
$env:VITE_DEV_PROXY_TARGET = "http://127.0.0.1:8080"
$env:VITE_DEV_PORT = "3000"
npm exec --yes --package pnpm@9.15.9 -- pnpm --dir frontend run dev -- --host 0.0.0.0 --port 3000
```

Restart backend when Go code changes:

```powershell
$pidFile = ".dev\pids\backend.pid"
if (Test-Path $pidFile) {
  $oldPid = [int](Get-Content $pidFile | Select-Object -First 1)
  Stop-Process -Id $oldPid -Force -ErrorAction SilentlyContinue
}

$backendLog = "$PWD\.dev\logs\backend.log"
$backendData = "$PWD\.dev\backend-data"
$go = "$PWD\.dev\tools\go\bin\go.exe"
$cmd = @"
`$env:DATA_DIR='$backendData'
`$env:SERVER_HOST='127.0.0.1'
`$env:SERVER_PORT='8080'
`$env:SERVER_MODE='debug'
& '$go' run ./cmd/server *> '$backendLog'
"@
$p = Start-Process -FilePath pwsh -ArgumentList @("-NoProfile","-ExecutionPolicy","Bypass","-Command",$cmd) -WorkingDirectory "$PWD\backend" -WindowStyle Hidden -PassThru
Set-Content -Path $pidFile -Value $p.Id
```

Stop backend:

```powershell
$pidFile = ".dev\pids\backend.pid"
if (Test-Path $pidFile) {
  $oldPid = [int](Get-Content $pidFile | Select-Object -First 1)
  Stop-Process -Id $oldPid -Force -ErrorAction SilentlyContinue
  Remove-Item $pidFile -Force
}
```

Check backend:

```powershell
Get-Content .dev\logs\backend.log -Tail 120
netstat -ano | Select-String ":8080"
```

Local runtime state is under `.dev/`.

## Code Change Workflow

After code edits, do not run frontend builds, backend builds, or deployment by default.
Only build or deploy when the user explicitly asks for it.

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
