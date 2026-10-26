# Cyklo – container edition

The same application as the SPK, packaged as a Docker image for anyone who does not have a Synology NAS. It runs on any x86_64 host that runs containers.

> **Provided "as is", without warranty of any kind** (MIT License). **The container edition is provided as a convenience and has not been tested by its author**, who runs the SPK on a Synology NAS. Reports of problems are welcome as GitHub issues. Make a backup (Settings → Backups) before every update.

## Security note

The app has no login of its own. Anyone who can reach the published port can read and change all data. Publish the port only on a trusted network (home LAN or a VPN such as Tailscale). Do not put it behind a public reverse proxy.

## Run with Docker Compose

1. Copy `docker-compose.yml` from this repository to the host.
2. Set `TZ` to your time zone (list of names: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).
3. Start it: `docker compose up -d`
4. Open `http://<host address>:8765/`.

Image: `ghcr.io/johnioan/cyklo` (tags: a version such as `0.1.8`, and `latest`). Platform: `linux/amd64` only.

## Settings

| Item | Value |
|---|---|
| Port | `8765` inside the container (change the left side of `8765:8765` to use another port on the host) |
| Data folder | `/data` (file `data.json`, plus one `.bak` copy); keep it on a volume so it survives updates |
| `TZ` | Time zone. It decides what "today" is and when reminders are sent. Default `UTC` if not set |
| `CYKLO_PORT`, `CYKLO_ADDR`, `CYKLO_DATA` | Optional; defaults suit the image |

The process runs as an unprivileged user (UID 10001). The compose file uses a named volume, which is created with the right ownership. If a host folder is used instead (bind mount), it must be writable by UID 10001, otherwise the app stops at start with "cannot open data".

## Update

1. Backup: Settings → Backups.
2. `docker compose pull && docker compose up -d`
3. The "What is new" window appears on the first visit.

## Moving from the SPK

Download a backup in the SPK version (Settings → Backups) and restore it in the container version. The backup file contains the Pushover keys unencrypted; store it safely.

## Differences from the SPK

- No DSM desktop icon or DSM login; the app is opened in a browser.
- Email reminders were not available in the SPK either; Pushover works the same way.

## Maintainer notes

- The image is built by `.github/workflows/docker.yml` when a tag `v<version>` is pushed (for example `v0.1.8`), and published to GitHub Container Registry.
- After the first publish, set the package visibility to **Public** (GitHub → the repository's Packages → Cyklo → Package settings), otherwise it cannot be pulled without logging in. The download count is shown on the package page.
- Local build: `docker build --build-arg VERSION=0.1.8 -t cyklo .`
