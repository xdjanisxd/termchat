#!/usr/bin/env bash

set -Eeuo pipefail

TARGET_COMMIT="${1:?commit SHA required}"

cd /srv/termchat

echo "==> Fetching repository"
git fetch origin master

echo "==> Checking commit"
git cat-file -e "${TARGET_COMMIT}^{commit}"

echo "==> Deploying commit: ${TARGET_COMMIT}"
git checkout master
git reset --hard "${TARGET_COMMIT}"

VERSION="$(git rev-parse --short=12 HEAD)"

echo "==> Version: ${VERSION}"

if grep -q '^TERMCHAT_IMAGE=' .env; then
    sed -i \
        "s|^TERMCHAT_IMAGE=.*|TERMCHAT_IMAGE=termchat-server:${VERSION}|" \
        .env
else
    printf '\nTERMCHAT_IMAGE=termchat-server:%s\n' "${VERSION}" >> .env
fi

echo "==> Validating compose"
docker compose config --quiet

echo "==> Building server image"
docker compose build --pull server

echo "==> Running migrations"
docker compose run --rm migrate

echo "==> Recreating server"
docker compose up \
    -d \
    --no-deps \
    --wait \
    server

echo "==> Checking containers"
docker compose ps -a

echo "==> Checking health"
curl -fsS http://127.0.0.1:8080/healthz

echo
echo "==> Deployment successful: ${VERSION}"
