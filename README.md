# APICalls

A self-hosted API testing platform (similar to Postman / Bruno / SoapUI), written in Go.

## Quick Start

### 1. Generate TLS certificate (first time only)
```
go run cmd/gencert/main.go
```
This creates `certs/server.crt` and `certs/server.key` (self-signed, valid 10 years).  
Replace these files with a CA-signed certificate for production use.

### 2. Build
```
go build -o apicalls.exe .
```

### 3. Run
```
./apicalls.exe
```

The server starts on **https://localhost:8443** by default (configure in `config/config.yaml`).

### 4. Login
Default credentials:
- **Username:** `admin`
- **Password:** `admin`

> Change the admin password immediately via the Users module after first login.

---

## Configuration

Edit `config/config.yaml`:

| Key | Default | Description |
|-----|---------|-------------|
| `server.port` | `8443` | HTTPS listen port |
| `server.host` | `0.0.0.0` | Bind address |
| `server.tls_cert` | `certs/server.crt` | TLS certificate path |
| `server.tls_key` | `certs/server.key` | TLS private key path |
| `database.path` | `data/apicalls.db` | SQLite database path |
| `session.secret` | (random string) | Cookie session secret — **change this!** |
| `log.path` | `logs/apicalls.log` | Audit log path |

---

## Roles

| Role | Environments | Variables | Technologies | Templates | Execution | Users |
|------|-------------|-----------|--------------|-----------|-----------|-------|
| **admin** | CRUD | CRUD + unmask | CRUD | CRUD | Run all | CRUD |
| **standard** | View | CRUD | CRUD | CRUD | Run all | — |
| **restricted** | View | View | View | View | Run non-restricted | — |
| **guest** | View | No access | View | View | No execution | — |

---

## Modules

- **Environments** — Define PRD / QMS / DEV environments with colour codes.
- **Variables** — Substitution variables (`{{VAR_NAME}}`) with per-environment values. Password masking.
- **Technologies** — Reusable HTTP blueprints (method, URL, headers, body, custom values).
- **Templates** — Request templates; support variable highlighting and technology import.
- **Execution** — Run templates against an environment. Notes panel on the right.
- **Users** — Admin-only user management.

---

## Logs

Audit entries are written to `logs/apicalls.log` in the format:
```
YYYY-MM-DD:HH:mm:ss | username             | module               | description
```

---

## Project Structure

```
APICalls/
├── main.go                  # Entry point, router
├── cmd/gencert/main.go      # Self-signed certificate generator
├── config/
│   ├── config.go            # Configuration loading
│   └── config.yaml          # Default configuration
├── auth/
│   ├── auth.go              # Session management, login/logout
│   └── middleware.go        # RequireLogin, RequireRole middlewares
├── models/                  # Data models (User, Environment, Variable, …)
├── storage/                 # SQLite CRUD operations (one file per entity)
├── handlers/                # HTTP handlers (one file per module)
├── logger/logger.go         # Audit logger
├── templates/               # HTML templates (Go html/template)
├── static/
│   ├── css/style.css
│   └── js/app.js
├── certs/                   # TLS certificate (generated)
├── data/                    # SQLite database (auto-created)
├── logs/                    # Audit log (auto-created)
└── prompts.txt              # Copilot prompt history
```
