# llsoai-websocket

A WebSocket and long-lived SSE bridge between AI assistants (browser / VSCode extension / desktop apps) and a managed backend. The repository ships a Go server, a Vue 3 web client, and a one-shot build script that embeds the front-end into a single static binary.

This document explains how to **configure** and **install** the project for local development and self-hosted deployment. It does **not** cover building the production binary &mdash; see `build.sh` for that.

---

## 1. Requirements

| Component | Version |
| --- | --- |
| Go | 1.24+ |
| Node.js | 18+ (20 LTS recommended) |
| npm | 9+ |
| MySQL | 5.7 / 8.0 |
| OS | macOS, Linux, Windows (WSL recommended) |

---

## 2. Repository layout

```
llsoai-websocket/
├── server/                 # Go backend (port 29081 by default)
│   ├── cmd/server/         # main entrypoint
│   ├── config.yaml         # backend configuration
│   └── internal/           # business logic
├── web/                    # Vue 3 front-end (Vite dev server on :5173)
├── build.sh                # production build script (not covered here)
└── README.md
```

---

## 3. Database setup

The backend stores user credentials and tokens in MySQL. Create a database and a user that match the values in `server/config.yaml`:

```sql
CREATE DATABASE openai DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE USER 'openai'@'%' IDENTIFIED BY 'your-strong-password';
GRANT ALL PRIVILEGES ON openai.* TO 'openai'@'%';
FLUSH PRIVILEGES;
```

Required tables (minimal schema used by `internal/store/user.go`):

```sql
USE openai;

CREATE TABLE IF NOT EXISTS users (
    id                    BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username              VARCHAR(64)  NOT NULL,
    password_hash         VARCHAR(255) NOT NULL,
    websocket_token       VARCHAR(128) DEFAULT NULL,
    websocket_token_hash  VARCHAR(255) DEFAULT NULL,
    created_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uniq_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

> If your installation uses additional columns (e.g. `email`, `display_name`), keep them &mdash; the server only reads the columns above.

---

## 4. Backend configuration

All backend settings live in `server/config.yaml`. The defaults shipped in the repo are suitable for local development.

### 4.1 Key fields

| Section | Field | Meaning |
| --- | --- | --- |
| `app` | `host`, `port` | Bind address. Default `0.0.0.0:29081`. |
| `database` | `host` / `port` / `username` / `password` / `name` | MySQL connection. Must match section 3. |
| `database` | `max_open_conns`, `max_idle_conns`, `conn_max_lifetime` | Connection pool tuning. |
| `websocket` | `path` | WebSocket endpoint, default `/ws`. |
| `websocket` | `read_timeout`, `write_timeout`, `heartbeat_timeout` | Socket-level timeouts. |
| `websocket` | `send_queue_size` | Outbound queue per client. |
| `http` | `request_timeout` | Max wall time for a chat request. |
| `http` | `history_timeout` | Max wait when fetching history from a WS client. |
| `http` | `sse_ping_interval` | Keep-alive interval for the long-lived workspace SSE stream. |
| `http` | `sse_max_lifetime` | Hard cap on a single SSE connection (clients auto-reconnect). |
| `http` | `max_history_response_bytes`, `max_history_message_bytes` | Safety caps. |
| `auth` | `api_keys` | Optional pre-shared keys for service-to-service calls. |
| `auth.jwt` | `secret` | **Change this in any non-local deployment.** |
| `auth.jwt` | `expiration` | Token lifetime, e.g. `720h` = 30 days. |
| `auth` | `bcrypt_cost` | Password hash cost. 10&ndash;12 is reasonable. |

### 4.2 Minimum changes before going live

1. Replace `database.password` with your real MySQL password.
2. Replace `auth.jwt.secret` with a long random string (`openssl rand -hex 32`).
3. If exposing the service publicly, put it behind HTTPS (Nginx, Caddy, etc.) and keep `app.host` as `0.0.0.0`.

### 4.3 Using a custom config path

```bash
./server -config /etc/llsoai/config.yaml
```

When unset, the server looks for `config.yaml` in the current working directory.

---

## 5. Front-end configuration

The web client lives in `web/` and reads its configuration from Vite environment variables and runtime defaults.

### 5.1 Dev-server proxy

`web/vite.config.ts` already proxies `/api`, `/health`, and `/ws` to the local backend on `http://127.0.0.1:29081`. No changes are needed when both server and client run on the same machine.

### 5.2 Pointing the client at a remote backend

Create `web/.env.local` (it is ignored by Git) and set the backend base URL:

```env
VITE_API_BASE_URL=https://api.example.com
VITE_WS_BASE_URL=wss://api.example.com
```

If those variables are absent, the client falls back to the current page origin, which is what you want when serving the embedded SPA from the Go binary.

### 5.3 Language

The UI supports `en-US`, `zh-CN`, `zh-TW`, `ja-JP`, `ko-KR`, `de-DE`, and `fr-FR`. The active language is resolved from:

1. The `?lang=` URL parameter
2. The `locale` key in `localStorage`
3. `en-US` (default fallback)

Users can switch languages from the language picker shown on the login page and inside the account settings dialog.

---

## 6. Installation

### 6.1 Clone

```bash
git clone <repo-url> llsoai-websocket
cd llsoai-websocket
```

### 6.2 Install backend dependencies

```bash
cd server
go mod download
cd ..
```

### 6.3 Install front-end dependencies

```bash
cd web
npm install
cd ..
```

> Tip: if you are in mainland China, set `GOPROXY=https://goproxy.cn,direct` for Go and use `npm config set registry https://registry.npmmirror.com` for npm.

### 6.4 Prepare MySQL

Run the SQL from [section 3](#3-database-setup), then update `server/config.yaml` with the matching credentials.

---

## 7. Running in development

Open two terminals.

**Terminal 1 &mdash; backend**

```bash
cd server
go run ./cmd/server -config config.yaml
```

You should see:

```
server listening on 0.0.0.0:29081
```

Health check:

```bash
curl http://127.0.0.1:29081/health
# -> ok
```

**Terminal 2 &mdash; front-end**

```bash
cd web
npm run dev
```

Open the URL printed by Vite (typically `http://127.0.0.1:5173`). The Vite dev server proxies API and WebSocket traffic to the backend, so both work out of the box.

---

## 8. Default endpoints

| Path | Type | Purpose |
| --- | --- | --- |
| `/health` | HTTP `GET` | Liveness probe |
| `/ws` | WebSocket | Persistent client channel (extensions, desktop) |
| `/api/auth/*` | HTTP | Login, registration, token management |
| `/api/workspaces/stream` | SSE | Long-lived per-user workspace event stream |
| `/api/chat/trigger` | HTTP `POST` | Start a chat run on a connected WS client |
| `/api/chat/cancel` | HTTP `POST` | Cancel an in-flight chat run |
| `/` | HTTP | Web UI (embedded in production binary) |

---

## 9. Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `connect database failed` on startup | Wrong credentials or MySQL not reachable | Verify `database.*` in `config.yaml`; test with `mysql -h ... -u ... -p` |
| `401` on every API call | JWT secret changed or token expired | Re-login from the web UI to get a fresh token |
| Front-end can reach `/api` but `/ws` fails | Reverse proxy strips `Upgrade` header | Enable WebSocket support in your proxy (Nginx: `proxy_set_header Upgrade $http_upgrade;`) |
| Language picker resets to English | `localStorage` cleared or `?lang=` not set | Pick the language again; the choice persists in `localStorage` |
| `address already in use` | Port `29081` taken | Change `app.port` in `config.yaml` or stop the conflicting process |

---

## 10. License

See repository for license information.
