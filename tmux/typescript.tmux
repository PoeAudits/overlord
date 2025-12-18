#!/usr/bin/env bash
# TypeScript project tmux layout
# MODE can be "override" (default) or "merge"
MODE=override
set -euo pipefail

S="$TMUX_SESSION"
ROOT="$TMUX_PROJECT_DIR"

# Rename the starter window (created by overlord-open)
tmux rename-window -t "${S}:base" "editor"
tmux send-keys -t "${S}:editor" "nvim" C-m

# Shell window
tmux new-window -t "$S:" -n shell -c "$ROOT"

# Git window
tmux new-window -t "$S:" -n git -c "$ROOT"
tmux send-keys -t "${S}:git" "git status" C-m

# --- TypeScript-specific options ---
# Development server
# tmux new-window -t "$S:" -n dev -c "$ROOT"
# tmux send-keys -t "${S}:dev" "pnpm dev" C-m

# Build in watch mode
# tmux new-window -t "$S:" -n build -c "$ROOT"
# tmux send-keys -t "${S}:build" "pnpm build --watch" C-m

# Type checking in watch mode
# tmux new-window -t "$S:" -n tsc -c "$ROOT"
# tmux send-keys -t "${S}:tsc" "pnpm tsc --watch --noEmit" C-m

# Run tests
# tmux new-window -t "$S:" -n test -c "$ROOT"
# tmux send-keys -t "${S}:test" "pnpm test" C-m

# Linting
# tmux new-window -t "$S:" -n lint -c "$ROOT"
# tmux send-keys -t "${S}:lint" "pnpm lint --watch" C-m

# Next.js / Vite specific
# tmux new-window -t "$S:" -n next -c "$ROOT"
# tmux send-keys -t "${S}:next" "pnpm next dev" C-m

tmux select-window -t "${S}:editor"
