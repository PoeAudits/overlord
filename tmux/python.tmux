#!/usr/bin/env bash
# Python project tmux layout
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

# --- Python-specific options ---
# Activate virtual environment in shell
# tmux send-keys -t "${S}:shell" "source .venv/bin/activate" C-m

# Run pytest in watch mode
# tmux new-window -t "$S:" -n test -c "$ROOT"
# tmux send-keys -t "${S}:test" "uv run pytest -v --tb=short" C-m

# Run development server (FastAPI/Flask)
# tmux new-window -t "$S:" -n server -c "$ROOT"
# tmux send-keys -t "${S}:server" "uv run uvicorn main:app --reload" C-m

# Python REPL
# tmux new-window -t "$S:" -n repl -c "$ROOT"
# tmux send-keys -t "${S}:repl" "uv run python" C-m

# Jupyter notebook
# tmux new-window -t "$S:" -n jupyter -c "$ROOT"
# tmux send-keys -t "${S}:jupyter" "uv run jupyter lab" C-m

tmux select-window -t "${S}:editor"
