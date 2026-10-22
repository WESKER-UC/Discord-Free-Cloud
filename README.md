# Discord Free Cloud

Turn your Discord servers into encrypted cloud storage with instant in-browser 4K media streaming.

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

![Dashboard Demo](demo.gif)

---

## Quickstart

```powershell
git clone https://github.com/WESKER-UC/Discord-Free-Cloud.git
cd Discord-Free-Cloud
.\start.bat
```

Double click `start.bat` anytime to launch the dashboard at `http://127.0.0.1:8080`.

---

## Architecture

The project supports local desktop use and cloud hosting. Cloud mode runs the existing Go application as one persistent Linux container:

```text
Browser ── HTTPS ──> Go web app (dashboard, API, WebSocket, background jobs)
                              ├── SQLite database on persistent host disk
                              └── Discord API (encrypted file chunks)
```

The frontend and backend are served from the same origin. Netlify's static hosting or serverless functions alone cannot run this application: it needs a persistent SQLite database, long-running background jobs, and WebSocket progress updates. You can use another static host for the frontend only if you separately configure a persistent backend and update the frontend API/WebSocket URLs; that split setup is not included.

## Cloud Deployment (Render)

The included `render.yaml` deploys the Linux container as a persistent Render web service. To deploy this repository:

1. In Render, create a Blueprint from `https://github.com/WESKER-UC/Discord-Free-Cloud` and select the `main` branch.
2. Apply the Blueprint. When prompted, set `APP_PASSWORD` to a unique secret of at least 16 characters. The HTTP Basic Auth username defaults to `admin`.
3. Open the deployed `onrender.com` URL, sign in with those HTTP Basic Auth credentials, then set a separate drive encryption password in the dashboard.
4. Configure your Discord bot and server as usual.

The service listens on the host-provided `PORT` and binds to `0.0.0.0`. Its persistent disk is mounted at `/var/data`; the SQLite database there stores the file catalog, bot configuration, and chunk metadata. Encrypted file chunks continue to be stored in Discord. Keep the disk attached, back it up, and keep a recoverable copy of the drive encryption password. Set `CLOUD_MODE=true`; the service refuses to start if `APP_PASSWORD` is missing or shorter than 16 characters.

Run one service instance. The current job queue, WebSocket clients, and SQLite database are local to that instance and are not shared across replicas. Hosting plans also impose practical upload-size, request-duration, temporary-disk, and bandwidth limits; this deployment does not make browser uploads unlimited.

### Cloud security and encryption

`APP_PASSWORD` protects the dashboard and API with HTTP Basic Auth; use the service over HTTPS. The separate drive password encrypts file chunks before they are sent to Discord. In cloud mode, the hosted backend receives the original upload and handles encryption and decryption; while the drive is unlocked, its key is held in backend memory. Therefore this is **not end-to-end encryption against the hosting provider**. Use a provider you trust, keep both passwords private, and do not expose an unprotected service to the public internet.

### Other persistent container hosts

Build with the included `Dockerfile`, mount a persistent volume at `/var/data`, and set `CLOUD_MODE=true`, `APP_PASSWORD` (at least 16 characters), `HOST=0.0.0.0`, `NO_BROWSER=true`, `NO_PROMPT=true`, and `DB_PATH=/var/data/drive.db`. The application reads `PORT` from the hosting platform.

## Local Use

On Windows, run `start.bat` and open `http://127.0.0.1:8080`. Linux and Android ARM64 users can build the Go application from source; on Android, Termux can run it locally, but the phone must stay online and awake while serving the dashboard.

---

## Cloud Storage Comparison

| Feature | Discord Free Cloud | Google Drive | Dropbox | Mega |
|---|---|---|---|---|
| **Monthly Price** | Depends on hosting and Discord plans | $9.99 / mo (2 TB) | $11.99 / mo (2 TB) | $10.99 / mo (2 TB) |
| **Total Storage** | Depends on Discord and hosting limits | 15 GB cap | 2 GB cap | 20 GB cap |
| **Max File Size** | Depends on host upload limits | 15 GB cap | 2 GB cap | 20 GB cap |
| **In-Browser 4K Video** | **Instant (Zero wait)** | Re-encodes video | Slow buffering | Limited |
| **Encryption Keys** | Your drive password; host can access it while unlocked | Google Staff | Dropbox Staff | Mega |
| **Multi-Server Redundancy** | **Yes** | No | No | No |
| **Telemetry & Tracking** | Uses your hosting provider and Discord | Ad tracking | Tracked | Tracked |

---

## Features

- **Chunked Uploads**: Splits files into 7.5 MB parts to fit the configured Discord upload flow. Your web host's upload, request, disk, and bandwidth limits still apply.
- **AES-256-GCM Encryption**: File chunks are encrypted before they are sent to Discord. In cloud deployment, the hosted backend handles encryption and holds the key in memory while the drive is unlocked, so use a cloud host you trust.
- **Instant In-Browser Streaming**: Watch 4K video, listen to music, view images, or read PDFs right inside the browser with fast timeline seeking.
- **Anti-Ban Protection**: Uses webhooks instead of the Bot API, cycles across storage channels with random jitter, and wraps chunks into valid PNG image containers.
- **Multi-Server Backup**: Replicate file chunks across multiple Discord servers. If one server is lost, your files remain safe on the others.
- **Instant Deduplication**: Re-uploading an existing file finishes in 1 second via SHA-256 hash matching.
- **Lightweight**: Uses ~15 MB RAM idle and ~40 MB during parallel transfers with reusable buffer pools.

---

## Performance Benchmarks

Measured on an AMD Ryzen 7 7435HS (8 Cores, 16 Threads, Windows 11, WiFi 6):

| Operation | Throughput | Notes |
|---|---|---|
| **AES-256 Encryption** | **1.86 GB/s** | Locks an 8 MB chunk in ~4.5 ms |
| **AES-256 Decryption** | **1.91 GB/s** | Unlocks an 8 MB chunk in ~4.3 ms |
| **PNG Container Wrapping** | **2.59 GB/s** | Formats encrypted bytes into valid PNG structure |
| **PNG Container Extraction** | **12.9 GB/s** | Pulls raw payload with zero overhead |
| **Upload Speed** | **25 – 38 MB/s** | Parallel worker pipeline across webhooks |
| **Download & Stream Speed** | **35 – 45 MB/s** | Fast chunk prefetching and range requests |
| **Idle Memory** | **~15 MB RAM** | Zero background memory bloat |

---

## Setup Guide

### 1. Create a Discord Bot
1. Go to the [Discord Developer Portal](https://discord.com/developers/applications) and create a **New Application**.
2. Under **Bot**, click **Reset Token**, copy your **Bot Token**, and enable **Message Content Intent**.
3. Under **OAuth2 > URL Generator**, select `bot` + `applications.commands` with `Administrator` permissions. Open the generated link to invite the bot to your server.

### 2. Get Your Server ID
1. In Discord settings, go to **Advanced** and enable **Developer Mode**.
2. Right-click your server icon and select **Copy Server ID**.

### 3. Connect & Upload
1. Open `http://127.0.0.1:8080` locally, or your cloud host's HTTPS URL and sign in with the HTTP Basic Auth credentials.
2. Set a drive encryption password (used to derive your AES-256 key; in cloud mode, the hosted backend handles the key while unlocked).
3. Paste your **Bot Token** and click **Verify Bot**, then click **Invite Bot** to link it to your server.
4. Drag and drop any files or folders to start uploading.

---

## Command Line Flags

```powershell
.\discord-free-cloud.exe [flags]
```

| Flag | Description | Default |
|---|---|---|
| `-port` | Web dashboard port | `8080` |
| `-db` | Database file location | `%APPDATA%\DiscordFreeCloud\drive.db` |
| `-password` | Encryption password (bypass prompt) | none |
| `-workers` | Number of parallel worker threads | `6` |
| `-no-browser` | Disable automatic browser launch | `false` |
| `-no-prompt` | Skip interactive console password prompt | `false` |

---

## FAQ

#### Can Discord see what I upload?
No. Every file chunk is encrypted with AES-256-GCM and polyglot-wrapped as a valid PNG image before transmission. Discord scanners only see standard image files with encrypted binary payloads.

#### What happens when Discord attachment links expire?
Discord attachment URLs expire after 24 hours. The engine automatically requests fresh authenticated attachment URLs on demand during downloads or streaming.

---

If this saved you time, leave a ⭐ to help others find it.
