# Terminal Chat Application — Archived Initial Product Brief

> **Historical document — not the current roadmap.** This file preserves the original English product direction. Repository code, tests, `README.md`, and `docs/` are authoritative. The source-reconciled roadmap is maintained in `C:/Users/gifted/Documents/hermes/Obsidian Vault/TermChat/Planning/08 - Roadmap Workspace.md`.

## Original product idea

The initial idea was a real-time, multi-user terminal chat application with user accounts, rooms, message history, and WebSocket communication. The intended experience resembled a compact Discord or IRC workflow inside a terminal: users register or log in, enter a room, read recent messages, send messages, and leave when finished.

The early brief also proposed a public lobby and room list, profiles, alternative technology stacks, local token storage, pre-created rooms, and numerous MVP TODOs. Those proposals are historical only and must not override the accepted product decisions.

## Current decisions that supersede this brief

- Rooms are private; there is no public room directory, lobby, or discovery feature.
- A user joins only with a known room name and a masked room-password prompt.
- Clients run natively on Windows amd64, Linux amd64, and Apple Silicon macOS arm64. Containers are for the server stack only.
- JWTs, passwords, invitation state, server selection, themes, and message history are not persisted to client disk.
- Direct chat is consent-based, target-specific, in-memory only, and expires after 60 seconds when unanswered.
- Room messages are stored for seven days; direct messages have no database persistence or history.
- User accounts, owned rooms, and related persistent data can be deleted through explicit account-deletion confirmation.
- PostgreSQL, migrations, and the Go server run in one root Compose stack; Nginx remains in a separate reverse-proxy LXC.
- The current security model includes Argon2id password hashing, JWT authentication, per-user message rate limits, and temporary failed-attempt locks.

## Historical exploration themes

The original exploration considered these broad areas:

- A terminal-first TUI with a message viewport, composer, command help, and online-user information.
- Account registration, login, password hashing, token-based authentication, and authenticated WebSocket sessions.
- Room creation, joining, message broadcasting, history loading, retention, reconnect behavior, rate limits, and input integrity.
- A Go server, PostgreSQL data store, REST API for authentication, and WebSocket transport for live events.
- Future ideas such as profiles, typing indicators, notifications, file sharing, moderation, reactions, message edits, end-to-end encryption, auto-update, and multi-replica deployment.

These themes informed the implemented system, but the historical implementation proposals and TODO lists are intentionally not an active plan. Consult the current documentation and roadmap before changing product behavior or architecture.
