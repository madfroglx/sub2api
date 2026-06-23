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

## Runtime Config Separation

Local development and server runtime intentionally use different config files.

Local development config:

- File: `.dev/backend-data/config.yaml`
- Database: local forwarded PostgreSQL port, `127.0.0.1:30332`
- Redis: remote Redis on `36.212.23.3:6379`

Server runtime config:

- File: `/opt/sub2api/config.yaml` on `36.212.23.3`
- Database: real PostgreSQL address, `10.175.15.123:30332`
- Redis: local Redis on the server, `127.0.0.1:6379`

Do not copy the local config file to the server during deployment. Runtime secrets and connection addresses stay in the environment-specific config files above.

## Publish Locally For Development

Use this when testing code changes on the local machine.

### macOS / Linux / Git Bash

```bash
make dev
make dev-restart
make dev-stop
make dev-status
make dev-logs
```

### Windows PowerShell

If services are already running, restart only the backend after Go code changes:

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

Start or restart the frontend dev server:

```powershell
$env:VITE_DEV_PROXY_TARGET = "http://127.0.0.1:8080"
$env:VITE_DEV_PORT = "3000"
npm exec --yes --package pnpm@9.15.9 -- pnpm --dir frontend run dev -- --host 0.0.0.0 --port 3000
```

Verify local services:

```powershell
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8080/api/v1/auth/me
Invoke-WebRequest -UseBasicParsing http://localhost:3000/
Get-Content .dev\logs\backend.log -Tail 120
netstat -ano | Select-String ":8080|:3000"
```

`/api/v1/auth/me` returning `401` is expected when not logged in.

## Publish To token-qs Server

Current production-like server:

- SSH target: `token-qs`
- Host: `36.212.23.3`
- Service: `sub2api`
- Runtime binary: `/opt/sub2api/sub2api`
- Runtime config: `/opt/sub2api/config.yaml`
- Source/build root: `/opt/sub2api-src`
- Public URL: `http://36.212.23.3:8000`

Preferred deployment path from Windows is local build plus binary upload. This avoids remote npm registry timeouts and avoids relying on Bash resolving the Windows SSH alias.

Build frontend locally:

```powershell
npm exec --yes --package pnpm@9.15.9 -- pnpm --dir frontend run build
```

Cross-compile the Linux backend binary locally:

```powershell
New-Item -ItemType Directory -Force .dev\build | Out-Null
Push-Location backend
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
..\.dev\tools\go\bin\go.exe build -tags embed -ldflags="-s -w -X main.BuildType=source -X main.Date=$(Get-Date -AsUTC -Format 'yyyy-MM-ddTHH:mm:ssZ')" -trimpath -o ..\.dev\build\sub2api-linux-amd64 ./cmd/server
Pop-Location
```

Upload and replace the server binary:

```powershell
$buildId = Get-Date -Format "yyyyMMddHHmmss"
scp .dev\build\sub2api-linux-amd64 token-qs:/tmp/sub2api.$buildId
ssh token-qs "sudo cp /opt/sub2api/sub2api /opt/sub2api/sub2api.backup.$buildId && sudo install -o sub2api -g sub2api -m 0755 /tmp/sub2api.$buildId /opt/sub2api/sub2api && sudo systemctl restart sub2api"
```

Verify server deployment:

```powershell
ssh token-qs "systemctl is-active sub2api"
ssh token-qs "sudo journalctl -u sub2api -n 100 --no-pager"
ssh token-qs "sudo ss -lntp | grep :8000"
Invoke-WebRequest -UseBasicParsing http://36.212.23.3:8000/
Invoke-WebRequest -UseBasicParsing http://36.212.23.3:8000/api/v1/auth/me
```

`/api/v1/auth/me` returning `401` is expected when not logged in.

Rollback to the previous binary if needed:

```powershell
ssh token-qs "ls -1t /opt/sub2api/sub2api.backup.* | head -n 1"
ssh token-qs "sudo cp /opt/sub2api/sub2api.backup.<build-id> /opt/sub2api/sub2api && sudo chown sub2api:sub2api /opt/sub2api/sub2api && sudo chmod 0755 /opt/sub2api/sub2api && sudo systemctl restart sub2api"
```

Replace `<build-id>` with the backup suffix selected by the first command.

## Deploy To sk

This path is kept for the legacy `sk` target. For `36.212.23.3`, prefer the `token-qs` deployment flow above.

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
