#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT_NAME="$(basename "$ROOT_DIR")"
PARENT_DIR="$(dirname "$ROOT_DIR")"

REMOTE_HOST="${SUB2API_DEPLOY_HOST:-sk}"
REMOTE_BASE="${SUB2API_REMOTE_BASE:-/opt/sub2api-src}"
REMOTE_APP_DIR="${SUB2API_REMOTE_APP_DIR:-/opt/sub2api}"
REMOTE_SERVICE="${SUB2API_REMOTE_SERVICE:-sub2api}"
GO_VERSION="${SUB2API_GO_VERSION:-1.26.4}"
PNPM_VERSION="${SUB2API_PNPM_VERSION:-10.15.1}"
BUILD_ID="${SUB2API_BUILD_ID:-$(date +%Y%m%d%H%M%S)}"
ARCHIVE="${TMPDIR:-/tmp}/sub2api-src-$BUILD_ID.tar.gz"
PUBLIC_URL="${SUB2API_PUBLIC_URL:-http://106.54.56.30:8000}"
BUILD_SWAP_SIZE="${SUB2API_BUILD_SWAP_SIZE:-4G}"

usage() {
  cat <<EOF
Usage:
  scripts/deploy-sk.sh

Environment:
  SUB2API_DEPLOY_HOST      SSH target, default: sk
  SUB2API_REMOTE_BASE      Remote source/build root, default: /opt/sub2api-src
  SUB2API_REMOTE_APP_DIR   Remote runtime app dir, default: /opt/sub2api
  SUB2API_REMOTE_SERVICE   systemd service name, default: sub2api
  SUB2API_GO_VERSION       Go version installed on remote if missing, default: 1.26.4
  SUB2API_PNPM_VERSION     pnpm version used through npm exec, default: 10.15.1
  SUB2API_BUILD_ID         Build id override, default: current timestamp
  SUB2API_PUBLIC_URL       HTTP verification URL, default: http://106.54.56.30:8000
  SUB2API_BUILD_SWAP_SIZE  Temporary remote swap size, default: 4G; set 0 to disable
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ $# -gt 0 ]]; then
  usage >&2
  exit 1
fi

echo "Deploy target: $REMOTE_HOST"
echo "Build id:      $BUILD_ID"
echo "Archive:       $ARCHIVE"

cleanup_local() {
  rm -f "$ARCHIVE"
}
trap cleanup_local EXIT

echo "Creating source archive..."
(
  cd "$PARENT_DIR"
  COPYFILE_DISABLE=1 tar -czf "$ARCHIVE" \
    --exclude "$PROJECT_NAME/.git" \
    --exclude "$PROJECT_NAME/.dev" \
    --exclude "$PROJECT_NAME/.codex" \
    --exclude "$PROJECT_NAME/.agents" \
    --exclude "$PROJECT_NAME/frontend/node_modules" \
    --exclude "$PROJECT_NAME/node_modules" \
    --exclude "$PROJECT_NAME/backend/bin" \
    --exclude "$PROJECT_NAME/backend/data" \
    --exclude "$PROJECT_NAME/backend/config.yaml" \
    --exclude "$PROJECT_NAME/backend/.installed" \
    --exclude "$PROJECT_NAME/deploy/.env" \
    --exclude "$PROJECT_NAME/deploy/config.yaml" \
    --exclude "$PROJECT_NAME/dist" \
    --exclude "$PROJECT_NAME/release" \
    --exclude "$PROJECT_NAME/**/._*" \
    --exclude "$PROJECT_NAME/._*" \
    "$PROJECT_NAME"
)

ls -lh "$ARCHIVE"
echo "Uploading source archive..."
scp "$ARCHIVE" "$REMOTE_HOST:/tmp/sub2api-src-$BUILD_ID.tar.gz"

echo "Building and publishing on remote..."
ssh "$REMOTE_HOST" \
  "BUILD_ID='$BUILD_ID' REMOTE_BASE='$REMOTE_BASE' REMOTE_APP_DIR='$REMOTE_APP_DIR' REMOTE_SERVICE='$REMOTE_SERVICE' GO_VERSION='$GO_VERSION' PNPM_VERSION='$PNPM_VERSION' PROJECT_NAME='$PROJECT_NAME' BUILD_SWAP_SIZE='$BUILD_SWAP_SIZE' bash -s" <<'REMOTE'
set -euo pipefail

RELEASES="$REMOTE_BASE/releases"
BUILD_DIR="$REMOTE_BASE/build"
CURRENT="$REMOTE_BASE/current"
ARCHIVE="/tmp/sub2api-src-$BUILD_ID.tar.gz"
EXTRACT_DIR="/tmp/sub2api-extract-$BUILD_ID"
SOURCE_DIR="$RELEASES/$BUILD_ID"
GO_ROOT="/opt/go$GO_VERSION"
GO_BIN="$GO_ROOT/bin/go"
NEW_BIN="$BUILD_DIR/sub2api.$BUILD_ID"
CLEAN_BIN="$BUILD_DIR/sub2api.clean"
SWAP_FILE="/tmp/sub2api-build-$BUILD_ID.swap"
SWAP_CREATED=0

cleanup_remote() {
  rm -f "$ARCHIVE"
  if [[ "$SWAP_CREATED" == "1" ]]; then
    sudo swapoff "$SWAP_FILE" >/dev/null 2>&1 || true
    sudo rm -f "$SWAP_FILE"
  fi
}
trap cleanup_remote EXIT

echo "Preparing remote directories..."
sudo mkdir -p "$RELEASES" "$BUILD_DIR"
sudo chown -R "$(id -un):$(id -gn)" "$REMOTE_BASE"
rm -rf "$EXTRACT_DIR"
mkdir -p "$EXTRACT_DIR"
tar -xzf "$ARCHIVE" -C "$EXTRACT_DIR"
rm -rf "$SOURCE_DIR"
mv "$EXTRACT_DIR/$PROJECT_NAME" "$SOURCE_DIR"
rm -rf "$EXTRACT_DIR"
ln -sfn "$SOURCE_DIR" "$CURRENT"

APPLEDOUBLE_COUNT="$(find "$SOURCE_DIR" -name '._*' | wc -l | tr -d ' ')"
if [[ "$APPLEDOUBLE_COUNT" != "0" ]]; then
  echo "Removing AppleDouble files: $APPLEDOUBLE_COUNT"
  find "$SOURCE_DIR" -name '._*' -delete
fi

if [[ ! -x "$GO_BIN" ]]; then
  echo "Installing Go $GO_VERSION..."
  TMP_GO="/tmp/go$GO_VERSION.linux-amd64.tar.gz"
  TMP_GO_DIR="/tmp/go-extract-$BUILD_ID"
  curl -L --connect-timeout 20 --max-time 180 -o "$TMP_GO" "https://go.dev/dl/go$GO_VERSION.linux-amd64.tar.gz"
  rm -rf "$TMP_GO_DIR"
  mkdir -p "$TMP_GO_DIR"
  tar -xzf "$TMP_GO" -C "$TMP_GO_DIR"
  sudo rm -rf "$GO_ROOT"
  sudo mv "$TMP_GO_DIR/go" "$GO_ROOT"
  rm -rf "$TMP_GO_DIR" "$TMP_GO"
fi

if [[ "$BUILD_SWAP_SIZE" != "0" && -z "$(swapon --show=NAME --noheadings | head -n 1)" ]]; then
  echo "Creating temporary build swap: $BUILD_SWAP_SIZE"
  sudo fallocate -l "$BUILD_SWAP_SIZE" "$SWAP_FILE"
  sudo chmod 600 "$SWAP_FILE"
  sudo mkswap "$SWAP_FILE" >/dev/null
  sudo swapon "$SWAP_FILE"
  SWAP_CREATED=1
fi

export PATH="$GO_ROOT/bin:$PATH"
echo "go:   $(go version)"
echo "node: $(node --version)"
echo "pnpm: $(npm exec --yes --package "pnpm@$PNPM_VERSION" -- pnpm --version)"

echo "Installing frontend dependencies..."
cd "$SOURCE_DIR"
npm exec --yes --package "pnpm@$PNPM_VERSION" -- pnpm --dir frontend install --frozen-lockfile

echo "Building frontend..."
npm exec --yes --package "pnpm@$PNPM_VERSION" -- pnpm --dir frontend run build

echo "Building backend binary..."
cd "$SOURCE_DIR/backend"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -tags embed \
  -ldflags="-s -w -X main.BuildType=source -X main.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -trimpath \
  -o "$NEW_BIN" \
  ./cmd/server
cp "$NEW_BIN" "$CLEAN_BIN"
chmod 0755 "$NEW_BIN" "$CLEAN_BIN"
"$NEW_BIN" --version 2>/dev/null || true

echo "Publishing binary..."
BACKUP="$REMOTE_APP_DIR/sub2api.backup.$BUILD_ID"
sudo cp "$REMOTE_APP_DIR/sub2api" "$BACKUP"
sudo install -o sub2api -g sub2api -m 0755 "$NEW_BIN" "$REMOTE_APP_DIR/sub2api.new"

rollback() {
  echo "Rolling back to $BACKUP" >&2
  sudo systemctl stop "$REMOTE_SERVICE" >/dev/null 2>&1 || true
  sudo cp "$BACKUP" "$REMOTE_APP_DIR/sub2api"
  sudo chown sub2api:sub2api "$REMOTE_APP_DIR/sub2api"
  sudo chmod 0755 "$REMOTE_APP_DIR/sub2api"
  sudo systemctl start "$REMOTE_SERVICE"
}

sudo systemctl stop "$REMOTE_SERVICE"
sudo mv "$REMOTE_APP_DIR/sub2api.new" "$REMOTE_APP_DIR/sub2api"
if ! sudo systemctl start "$REMOTE_SERVICE"; then
  rollback
  exit 1
fi

sleep 5
if ! systemctl is-active --quiet "$REMOTE_SERVICE"; then
  rollback
  exit 1
fi

echo "Deployment successful."
echo "source:  $SOURCE_DIR"
echo "binary:  $REMOTE_APP_DIR/sub2api"
echo "backup:  $BACKUP"
echo "service: $(systemctl is-active "$REMOTE_SERVICE")"
sudo ss -lntp | grep ':8000' || true
sudo "$REMOTE_APP_DIR/sub2api" --version 2>/dev/null || true
REMOTE

echo "Verifying HTTP..."
curl --noproxy '*' -sS -i "$PUBLIC_URL/api/v1/auth/me" | sed -n '1,20p'
curl --noproxy '*' -sS -i "$PUBLIC_URL/" | sed -n '1,12p'

echo "Done."
