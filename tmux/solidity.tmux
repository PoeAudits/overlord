#!/usr/bin/env bash
# Solidity project tmux layout
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

# --- Solidity/Foundry-specific options ---
# Run local Anvil node
# tmux new-window -t "$S:" -n anvil -c "$ROOT"
# tmux send-keys -t "${S}:anvil" "anvil" C-m

# Run forge tests in watch mode
# tmux new-window -t "$S:" -n test -c "$ROOT"
# tmux send-keys -t "${S}:test" "forge test --watch" C-m

# Build contracts
# tmux new-window -t "$S:" -n build -c "$ROOT"
# tmux send-keys -t "${S}:build" "forge build" C-m

# Gas snapshots
# tmux new-window -t "$S:" -n gas -c "$ROOT"
# tmux send-keys -t "${S}:gas" "forge snapshot" C-m

# Cast interactions
# tmux new-window -t "$S:" -n cast -c "$ROOT"

# Slither analysis
# tmux new-window -t "$S:" -n slither -c "$ROOT"
# tmux send-keys -t "${S}:slither" "slither ." C-m

tmux select-window -t "${S}:editor"
