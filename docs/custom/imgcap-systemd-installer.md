# imgcap Systemd Installer

Last updated: 2026-05-31

## Purpose

`deploy/install-imgcap.sh` is the one-click binary installer for the customized imgcap build. It mirrors the official Sub2API binary/systemd installation style while keeping the custom release source, version label, and external database/cache assumptions explicit.

This path is for servers where PostgreSQL and Redis are already installed, such as a 1Panel-managed host. It does not install PostgreSQL, Redis, Docker, or Docker Compose.

## Installer Entry

- Script: `deploy/install-imgcap.sh`
- Default release repo: `lsmallice/sub2api`
- Default release asset pattern: `sub2api_<version>_<os>_<arch>.tar.gz`
- Default install dir: `/opt/sub2api`
- Default config dir: `/etc/sub2api`
- Default service: `sub2api`
- Default data dir: `/opt/sub2api/data`

Typical install:

```bash
curl -sSL https://raw.githubusercontent.com/lsmallice/sub2api/image-capability/deploy/install-imgcap.sh | sudo bash
```

Install a fixed release:

```bash
curl -sSL https://raw.githubusercontent.com/lsmallice/sub2api/image-capability/deploy/install-imgcap.sh | sudo bash -s -- --version imgcap-0.1.133
```

Install from a local binary or archive:

```bash
sudo bash deploy/install-imgcap.sh --binary /tmp/sub2api
sudo bash deploy/install-imgcap.sh --archive /tmp/sub2api_0.1.133_linux_amd64.tar.gz
```

## 1Panel PostgreSQL And Redis

The installer assumes PostgreSQL and Redis already exist. For a 1Panel local deployment, the default host addresses are:

```text
DATABASE_HOST=127.0.0.1
DATABASE_PORT=5432
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
```

Set the 1Panel database and Redis credentials in the web setup wizard. In default wizard mode, `deploy/install-imgcap.sh` intentionally does not persist `DATABASE_*` or `REDIS_*` into `/etc/sub2api/sub2api.env`, because those environment variables override `DATA_DIR/config.yaml` at runtime.

For unattended setup only, pass the 1Panel database and Redis credentials through environment variables when running the script:

```bash
curl -sSL https://raw.githubusercontent.com/lsmallice/sub2api/image-capability/deploy/install-imgcap.sh \
  | sudo AUTO_SETUP=true \
      DATABASE_USER=sub2api \
      DATABASE_PASSWORD='***' \
      DATABASE_DBNAME=sub2api \
      REDIS_PASSWORD='***' \
      bash
```

When `AUTO_SETUP=true`, the generated `/etc/sub2api/sub2api.env` stores these values with mode `0600`. Do not print this file in logs or chat.

## Setup Modes

Default mode leaves `AUTO_SETUP=false`, starts the web setup flow, and lets the administrator finish database, Redis, and admin account setup through the browser.

For unattended setup, pass `AUTO_SETUP=true` plus complete DB, Redis, and admin credentials:

```bash
sudo AUTO_SETUP=true \
  ADMIN_EMAIL=admin@example.com \
  ADMIN_PASSWORD='***' \
  DATABASE_USER=sub2api \
  DATABASE_PASSWORD='***' \
  DATABASE_DBNAME=sub2api \
  REDIS_PASSWORD='***' \
  bash deploy/install-imgcap.sh
```

Use unattended setup only when credentials are known and the target database is intended for this Sub2API instance.

## Systemd Behavior

The script writes `/etc/systemd/system/sub2api.service` with:

- `EnvironmentFile=-/etc/sub2api/sub2api.env`
- `WorkingDirectory=/opt/sub2api`
- `ExecStart=/opt/sub2api/sub2api`
- `ReadWritePaths=/opt/sub2api /opt/sub2api/data`
- `ProtectSystem=strict`

The app writes `config.yaml` and setup lock files under `DATA_DIR`, which defaults to `/opt/sub2api/data`.

Useful commands:

```bash
systemctl status sub2api
journalctl -u sub2api -f
systemctl restart sub2api
```

## Upgrade And Rollback

Before replacing an existing binary, the installer copies it to:

```text
/opt/sub2api/backups/sub2api-<timestamp>
```

Upgrade to the latest release from the custom repo:

```bash
sudo bash deploy/install-imgcap.sh upgrade
```

Upgrade to a fixed release:

```bash
sudo bash deploy/install-imgcap.sh upgrade --version imgcap-0.1.133
```

Rollback manually by restoring a backup binary and restarting:

```bash
sudo systemctl stop sub2api
sudo install -m 0755 -o sub2api -g sub2api /opt/sub2api/backups/sub2api-YYYYMMDD-HHMMSS /opt/sub2api/sub2api
sudo systemctl start sub2api
```

Database migrations are still forward-only. Because this installer points at existing PostgreSQL, take a database backup before releases that include migrations.

## Release Asset Requirements

The GitHub release must include the same asset style as the official binary installer:

```text
sub2api_<version>_linux_amd64.tar.gz
sub2api_<version>_linux_arm64.tar.gz
checksums.txt
```

The repo provides `.github/workflows/imgcap-binary-release.yml` to build and upload these assets automatically. It runs on tags matching `imgcap-*` and can also be run manually from GitHub Actions.

Manual workflow inputs:

- `tag`: release tag, for example `imgcap-0.1.133`
- `base_version`: upstream base version, for example `0.1.133`
- `prerelease`: whether to mark the GitHub Release as prerelease

If `base_version` is omitted, the workflow reads `backend/cmd/server/VERSION`.

The binary should be built with custom version flags:

```text
Version=<upstream-base-version>
BuildLabel=imgcap-<upstream-base-version>
BuildType=custom
Commit=<shortsha>
```

`Version` remains the upstream base semver for update comparison. `BuildLabel` is the UI display label.

## Upstream Merge Checklist

After merging official `main`, verify:

- `deploy/install-imgcap.sh` still uses the custom release repo and does not fall back to `weishaw/sub2api:latest`.
- The official release asset naming has not changed. If it changed, update the installer and this document.
- The systemd unit still matches runtime needs, especially `DATA_DIR`, setup mode, and writable paths.
- Custom version flags still exist in `backend/cmd/server/main.go`.
- The installer remains Docker-free.

Regression commands:

```bash
bash -n deploy/install-imgcap.sh
bash deploy/install-imgcap.sh --help
```

GitHub Actions regression:

- Run `imgcap Binary Release` manually with a test tag.
- Confirm the Release Assets include both Linux tarballs and `checksums.txt`.
- Confirm `install-imgcap.sh --version <tag>` resolves the asset name using the base version, not the `imgcap-*` tag prefix.
