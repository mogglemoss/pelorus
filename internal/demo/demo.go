// Package demo creates a self-contained temporary filesystem for demo
// recordings and screenshots. Call Setup() to get the root path; call the
// returned cleanup func when done (defer it in main).
package demo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Setup creates a demo directory tree under the user's home directory and
// returns its path plus a cleanup function. Using $HOME keeps the breadcrumb
// path short (compresses to ~ › pelorus-demo › axiom) for clean recordings.
func Setup() (root string, cleanup func(), err error) {
	home, herr := os.UserHomeDir()
	if herr != nil {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.TempDir()
	}
	root = filepath.Join(home, "pelorus-demo")
	if err = os.RemoveAll(root); err != nil {
		return "", nil, fmt.Errorf("demo: clear existing demo dir: %w", err)
	}
	if err = os.MkdirAll(root, 0o755); err != nil {
		return "", nil, fmt.Errorf("demo: create demo dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(root) }

	if err = buildTree(root); err != nil {
		cleanup()
		return "", nil, err
	}
	return root, cleanup, nil
}

func buildTree(root string) error {
	// Directory skeleton.
	dirs := []string{
		"axiom",
		"axiom/cmd",
		"axiom/internal/api",
		"axiom/internal/auth",
		"axiom/internal/config",
		"axiom/internal/models",
		"axiom/internal/store",
		"axiom/web/src/components",
		"axiom/web/src/hooks",
		"axiom/web/public",
		"axiom/scripts",
		"axiom/docs",
		"notes",
		"scratch",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return err
		}
	}

	files := map[string]string{
		// Go files
		"axiom/cmd/main.go": goMain,
		"axiom/internal/api/server.go":      goServer,
		"axiom/internal/api/routes.go":      goRoutes,
		"axiom/internal/auth/middleware.go": goMiddleware,
		"axiom/internal/config/config.go":   goConfig,
		"axiom/internal/models/user.go":     goUser,
		"axiom/internal/store/postgres.go":  goPostgres,
		"axiom/go.mod":                      goMod,
		"axiom/go.sum":                      goSum,
		"axiom/Makefile":                    makefile,

		// TypeScript / React
		"axiom/web/src/components/Dashboard.tsx": tsDashboard,
		"axiom/web/src/components/Sidebar.tsx":   tsSidebar,
		"axiom/web/src/hooks/useAuth.ts":         tsUseAuth,
		"axiom/web/package.json":                 packageJSON,
		"axiom/web/tsconfig.json":                tsconfig,

		// Markdown
		"axiom/README.md":        readmeMd,
		"axiom/docs/api.md":      apiMd,
		"axiom/docs/deploy.md":   deployMd,

		// Config / shell
		"axiom/scripts/setup.sh":  setupSh,
		"axiom/.env.example":      envExample,
		"axiom/.gitignore":        gitignore,
		"axiom/docker-compose.yml": dockerCompose,

		// Notes
		"notes/ideas.md":    ideasMd,
		"notes/meetings.md": meetingsMd,

		// Scratch
		"scratch/test.py":  testPy,
		"scratch/query.sql": querySql,
	}

	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
	}

	// Init a git repo inside axiom/ so git-status glyphs are visible.
	axiomDir := filepath.Join(root, "axiom")
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "demo@pelorus.dev"},
		{"config", "user.name", "Demo User"},
		{"add", "."},
		{"commit", "-m", "initial commit"},
	} {
		c := exec.Command("git", args...)
		c.Dir = axiomDir
		_ = c.Run()
	}

	// Make a couple of files "modified" so M glyphs appear.
	_ = appendLine(filepath.Join(axiomDir, "internal/api/server.go"), "\n// TODO: add rate limiting\n")
	_ = appendLine(filepath.Join(axiomDir, "internal/models/user.go"), "\n// TODO: add avatar field\n")

	// Add an untracked file so ? glyph appears.
	_ = os.WriteFile(filepath.Join(axiomDir, "internal/api/websocket.go"), []byte(goWebsocket), 0o644)

	return nil
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

// ---------------------------------------------------------------------------
// File contents
// ---------------------------------------------------------------------------

const goMain = `package main

import (
	"log"
	"os"

	"github.com/axiom/cmd/server"
)

func main() {
	if err := server.Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
`

const goServer = `package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/axiom/internal/config"
	"github.com/axiom/internal/auth"
)

type Server struct {
	cfg    *config.Config
	router *http.ServeMux
	auth   *auth.Middleware
}

func New(cfg *config.Config) *Server {
	s := &Server{cfg: cfg, router: http.NewServeMux()}
	s.registerRoutes()
	return s
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	return srv.ListenAndServe()
}
`

const goRoutes = `package api

import "net/http"

func (s *Server) registerRoutes() {
	s.router.Handle("GET /health",     http.HandlerFunc(s.handleHealth))
	s.router.Handle("GET /api/users",  s.auth.Wrap(http.HandlerFunc(s.handleListUsers)))
	s.router.Handle("POST /api/users", s.auth.Wrap(http.HandlerFunc(s.handleCreateUser)))
	s.router.Handle("GET /api/users/{id}", s.auth.Wrap(http.HandlerFunc(s.handleGetUser)))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
`

const goMiddleware = `package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userKey contextKey = "user"

type Middleware struct {
	secret string
}

func New(secret string) *Middleware {
	return &Middleware{secret: secret}
}

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !m.validate(token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) validate(token string) bool {
	return token != "" && len(token) > 16
}
`

const goConfig = `package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port     int
	DSN      string
	Secret   string
	LogLevel string
}

func Load() *Config {
	port, _ := strconv.Atoi(env("PORT", "8080"))
	return &Config{
		Port:     port,
		DSN:      env("DATABASE_URL", "postgres://localhost/axiom"),
		Secret:   env("JWT_SECRET", "change-me"),
		LogLevel: env("LOG_LEVEL", "info"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
`

const goUser = `package models

import "time"

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

type User struct {
	ID        int64     ` + "`" + `db:"id"` + "`" + `
	Email     string    ` + "`" + `db:"email"` + "`" + `
	Name      string    ` + "`" + `db:"name"` + "`" + `
	Role      Role      ` + "`" + `db:"role"` + "`" + `
	CreatedAt time.Time ` + "`" + `db:"created_at"` + "`" + `
	UpdatedAt time.Time ` + "`" + `db:"updated_at"` + "`" + `
}
`

const goPostgres = `package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/axiom/internal/models"
)

type Store struct {
	db *sql.DB
}

func New(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) GetUser(ctx context.Context, id int64) (*models.User, error) {
	u := &models.User{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, email, name, role, created_at, updated_at FROM users WHERE id = $1", id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}
`

const goWebsocket = `package api

import "net/http"

// TODO: implement WebSocket support
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
`

const goMod = `module github.com/axiom

go 1.23

require (
	github.com/lib/pq v1.10.9
)
`

const goSum = `github.com/lib/pq v1.10.9 h1:YXG7RB+JIjhP29X+OtkiDnYaXQwpS3htCVWkH2BJSDY=
github.com/lib/pq v1.10.9/go.mod h1:AlVN5x4E4T544tWzH6hKfbfQvm3HdbOxrmggDmAdpxY=
`

const makefile = `.PHONY: build run test lint

build:
	go build -o bin/axiom ./cmd

run: build
	./bin/axiom

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

migrate:
	goose -dir migrations postgres "$(DATABASE_URL)" up
`

const tsDashboard = `import React, { useEffect, useState } from 'react'
import { useAuth } from '../hooks/useAuth'
import { Sidebar } from './Sidebar'

interface Metric {
  label: string
  value: number
  delta: number
}

export const Dashboard: React.FC = () => {
  const { user } = useAuth()
  const [metrics, setMetrics] = useState<Metric[]>([])

  useEffect(() => {
    fetch('/api/metrics')
      .then(r => r.json())
      .then(setMetrics)
  }, [])

  return (
    <div className="flex h-screen bg-gray-950">
      <Sidebar />
      <main className="flex-1 p-8 overflow-auto">
        <h1 className="text-2xl font-semibold text-white mb-6">
          Welcome back, {user?.name}
        </h1>
        <div className="grid grid-cols-3 gap-6">
          {metrics.map(m => (
            <MetricCard key={m.label} {...m} />
          ))}
        </div>
      </main>
    </div>
  )
}
`

const tsSidebar = `import React from 'react'
import { NavLink } from 'react-router-dom'

const links = [
  { to: '/',        label: 'Dashboard' },
  { to: '/users',   label: 'Users'     },
  { to: '/reports', label: 'Reports'   },
  { to: '/settings',label: 'Settings'  },
]

export const Sidebar: React.FC = () => (
  <aside className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col py-6">
    <div className="px-4 mb-8 text-lg font-bold text-white">Axiom</div>
    <nav className="flex-1 space-y-1 px-2">
      {links.map(({ to, label }) => (
        <NavLink key={to} to={to}
          className={({ isActive }) =>
            'flex items-center px-3 py-2 rounded-md text-sm ' +
            (isActive ? 'bg-indigo-600 text-white' : 'text-gray-400 hover:text-white')
          }
        >
          {label}
        </NavLink>
      ))}
    </nav>
  </aside>
)
`

const tsUseAuth = `import { useContext } from 'react'
import { AuthContext } from '../context/AuthContext'

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
`

const packageJSON = `{
  "name": "axiom-web",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev":   "vite",
    "build": "tsc && vite build",
    "lint":  "eslint src --ext .ts,.tsx"
  },
  "dependencies": {
    "react":        "^18.3.1",
    "react-dom":    "^18.3.1",
    "react-router-dom": "^6.26.0"
  },
  "devDependencies": {
    "@types/react":     "^18.3.5",
    "@types/react-dom": "^18.3.0",
    "typescript":       "^5.5.3",
    "vite":             "^5.4.0"
  }
}
`

const tsconfig = `{
  "compilerOptions": {
    "target": "ES2020",
    "lib": ["ES2020", "DOM"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true
  },
  "include": ["src"]
}
`

const readmeMd = `# Axiom

A lightweight web application framework for Go with a React frontend.

## Features

- **Fast** — zero-allocation routing, connection pooling, compiled templates
- **Type-safe** — end-to-end TypeScript with generated API clients
- **Observable** — structured JSON logs, Prometheus metrics, distributed tracing
- **Deployable** — single binary, Docker image < 20 MB, Kubernetes-ready

## Quick start

` + "```" + `bash
git clone https://github.com/example/axiom
cd axiom
make run
` + "```" + `

Navigate to ` + "`" + `http://localhost:8080` + "`" + `.

## Architecture

` + "```" + `
cmd/
  main.go          ← entry point
internal/
  api/             ← HTTP handlers and routing
  auth/            ← JWT middleware
  config/          ← environment-based config
  models/          ← domain types
  store/           ← PostgreSQL repository
web/
  src/             ← React + TypeScript frontend
` + "```" + `

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8080 | HTTP listen port |
| DATABASE_URL | postgres://localhost/axiom | PostgreSQL DSN |
| JWT_SECRET | — | Secret key for JWT signing |
| LOG_LEVEL | info | Log verbosity (debug/info/warn/error) |

## License

MIT
`

const apiMd = `# API Reference

## Authentication

All endpoints except ` + "`GET /health`" + ` require a Bearer token.

` + "```" + `
Authorization: Bearer <token>
` + "```" + `

## Endpoints

### GET /health

Returns ` + "`200 OK`" + ` when the server is running.

### GET /api/users

Returns a paginated list of users.

**Query parameters**

| Name | Type | Default | Description |
|------|------|---------|-------------|
| page | int | 1 | Page number |
| limit | int | 20 | Items per page |
| role | string | — | Filter by role |

### POST /api/users

Creates a new user.
`

const deployMd = `# Deployment

## Docker

` + "```" + `bash
docker build -t axiom:latest .
docker run -p 8080:8080 \
  -e DATABASE_URL=postgres://... \
  -e JWT_SECRET=... \
  axiom:latest
` + "```" + `

## Kubernetes

` + "```" + `bash
kubectl apply -f k8s/
` + "```" + `

## Environment variables

Set ` + "`DATABASE_URL`" + ` and ` + "`JWT_SECRET`" + ` as Kubernetes secrets.
`

const setupSh = `#!/usr/bin/env bash
set -euo pipefail

echo "Setting up Axiom development environment..."

# Check dependencies
for cmd in go node psql; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "Error: $cmd is required but not installed." >&2
    exit 1
  fi
done

# Create database
createdb axiom 2>/dev/null || echo "Database already exists"

# Install Go dependencies
go mod download

# Install Node dependencies
cd web && npm install && cd ..

echo "Done. Run 'make run' to start the server."
`

const envExample = `# Copy to .env and fill in values
PORT=8080
DATABASE_URL=postgres://localhost/axiom?sslmode=disable
JWT_SECRET=change-me-in-production
LOG_LEVEL=info
`

const gitignore = `.env
bin/
dist/
node_modules/
*.test
coverage/
`

const dockerCompose = `services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://postgres:postgres@db/axiom?sslmode=disable
      JWT_SECRET: dev-secret
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: axiom
      POSTGRES_PASSWORD: postgres
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5
`

const ideasMd = `# Ideas

## Product
- [ ] Real-time collaboration via WebSockets
- [ ] Plugin system for custom data sources
- [ ] Mobile app (React Native)

## Tech
- [ ] Replace PostgreSQL with TigerBeetle for financials
- [ ] WASM-compiled core for edge deployments
- [ ] End-to-end encryption at rest

## Business
- [ ] Self-hosted tier with license key
- [ ] Usage-based pricing above free tier
`

const meetingsMd = `# Meeting Notes

## 2026-04-10 — Sprint planning

Attendees: Alice, Bob, Carol

- Agreed to ship WebSocket endpoint by end of sprint
- Auth refresh tokens deferred to next quarter
- Infra: migrate to arm64 instances to cut costs ~40%

## 2026-04-03 — Design review

- New dashboard layout approved
- Sidebar collapsed by default on mobile
- Accessibility pass needed before launch
`

const testPy = `#!/usr/bin/env python3
"""Quick load test for the API."""

import asyncio
import aiohttp
import time

BASE = "http://localhost:8080"
CONCURRENCY = 50
REQUESTS = 1000

async def hit(session: aiohttp.ClientSession, url: str) -> float:
    t0 = time.monotonic()
    async with session.get(url) as r:
        await r.read()
    return time.monotonic() - t0

async def main() -> None:
    connector = aiohttp.TCPConnector(limit=CONCURRENCY)
    async with aiohttp.ClientSession(connector=connector) as session:
        tasks = [hit(session, f"{BASE}/health") for _ in range(REQUESTS)]
        latencies = await asyncio.gather(*tasks)
    avg = sum(latencies) / len(latencies) * 1000
    p99 = sorted(latencies)[int(len(latencies) * 0.99)] * 1000
    print(f"avg={avg:.1f}ms  p99={p99:.1f}ms  rps={REQUESTS/sum(latencies)*CONCURRENCY:.0f}")

if __name__ == "__main__":
    asyncio.run(main())
`

const querySql = `-- Active users by role in the last 30 days
SELECT
    u.role,
    COUNT(DISTINCT u.id)           AS user_count,
    COUNT(e.id)                    AS event_count,
    ROUND(AVG(e.duration_ms), 1)   AS avg_duration_ms
FROM users u
JOIN events e ON e.user_id = u.id
WHERE e.created_at >= NOW() - INTERVAL '30 days'
GROUP BY u.role
ORDER BY event_count DESC;
`
