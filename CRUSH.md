# CRUSH

This file provides guidelines for working with the Controly monorepo.

## Commands

### Go (Backend)

- **Format:** `go fmt ./...`
- **Build:** `go build ./...`
- **Test:** `go test ./...`
- **Run:** `go run ./...`

### TypeScript/Node.js (Frontend & SDK)

- **Install:** Run `pnpm install` in the specific project directory (`sdk`, `server/controller`, etc.).
- **Develop:** Run `pnpm dev` in the specific project directory.
- **Build:** Run `pnpm build` in the specific project directory.

## Code Style

### General

- This is a monorepo with multiple Go and TypeScript projects.
- Use `pnpm` for managing Node.js dependencies. Do not delete `pnpm-*.yaml` files.
- Do not modify `package.json` files directly. Use `pnpm` commands.

### Go

- Format all Go code with `go fmt` before committing.
- Check for build errors with `go build` after formatting.

### TypeScript

- Follow standard TypeScript best practices.
- Naming conventions: `camelCase` for variables and functions, `PascalCase` for classes and types.
- Styling is done with `daisyui` and `tailwindcss`.
