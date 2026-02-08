# ==============================================================================
# Overlord Makefile
# ==============================================================================

.PHONY: help deps build install deploy run test bench fmt vet clean \
		 setup config init list info detect new add rm clear archive unarchive open \
		 activate deactivate sync doctor backup sync-lock-show sync-lock-transfer sync-lock-claim env-pull env-generate \
        wt-new wt-list wt-attach wt-remove \
        tmux-send tmux-read tmux-list

# ==============================================================================
# Tmux Utilities (current session)
# ==============================================================================

# Send command to window in current tmux session
# Usage: make tmux-send WINDOW=<window> CMD="<command>"
tmux-send:
ifndef WINDOW
	$(error WINDOW is required. Usage: make tmux-send WINDOW=<window> CMD="<command>")
endif
ifndef CMD
	$(error CMD is required. Usage: make tmux-send WINDOW=<window> CMD="<command>")
endif
	@if [ -z "$$TMUX" ]; then \
		echo "Error: Not running inside tmux session"; \
		echo ""; \
		echo "This command must be run from within a tmux session."; \
		echo "Use 'overlord open' or 'overlord worktree attach' to enter a session."; \
		exit 1; \
	fi; \
	\
	SESSION=$$(tmux display-message -p '#S'); \
	WINDOW_NAME="$(WINDOW)"; \
	COMMAND="$(CMD)"; \
	\
	if ! tmux list-windows -t "$$SESSION" 2>/dev/null | grep -q "^[0-9]*: $$WINDOW_NAME"; then \
		echo "Error: Window '$$WINDOW_NAME' does not exist in current session '$$SESSION'"; \
		echo ""; \
		echo "Available windows:"; \
		tmux list-windows -t "$$SESSION" 2>/dev/null | awk '{print "  " $$2}' | sed 's/\*$$//'; \
		exit 1; \
	fi; \
	\
	echo "Sending to $$SESSION:$$WINDOW_NAME: $$COMMAND"; \
	tmux send-keys -t "$$SESSION:$$WINDOW_NAME" "$$COMMAND" C-m

# Read visible pane content from window in current tmux session
# Usage: make tmux-read WINDOW=<window>
tmux-read:
ifndef WINDOW
	$(error WINDOW is required. Usage: make tmux-read WINDOW=<window>)
endif
	@if [ -z "$$TMUX" ]; then \
		echo "Error: Not running inside tmux session"; \
		echo ""; \
		echo "This command must be run from within a tmux session."; \
		echo "Use 'overlord open' or 'overlord worktree attach' to enter a session."; \
		exit 1; \
	fi; \
	\
	SESSION=$$(tmux display-message -p '#S'); \
	WINDOW_NAME="$(WINDOW)"; \
	\
	if ! tmux list-windows -t "$$SESSION" 2>/dev/null | grep -q "^[0-9]*: $$WINDOW_NAME"; then \
		echo "Error: Window '$$WINDOW_NAME' does not exist in current session '$$SESSION'"; \
		echo ""; \
		echo "Available windows:"; \
		tmux list-windows -t "$$SESSION" 2>/dev/null | awk '{print "  " $$2}' | sed 's/\*$$//'; \
		exit 1; \
	fi; \
	\
	echo "Reading from $$SESSION:$$WINDOW_NAME:"; \
	echo "----------------------------------------"; \
	tmux capture-pane -t "$$SESSION:$$WINDOW_NAME" -p

# List all windows in current tmux session
# Usage: make tmux-list
tmux-list:
	@if [ -z "$$TMUX" ]; then \
		echo "Error: Not running inside tmux session"; \
		echo ""; \
		echo "This command must be run from within a tmux session."; \
		echo "Use 'overlord open' or 'overlord worktree attach' to enter a session."; \
		exit 1; \
	fi; \
	\
	SESSION=$$(tmux display-message -p '#S'); \
	echo "Windows in session '$$SESSION':"; \
	echo ""; \
	tmux list-windows -t "$$SESSION" | while IFS=: read -r index rest; do \
		NAME=$$(echo "$$rest" | awk '{print $$1}' | sed 's/\*$$//'); \
		IS_ACTIVE=$$(echo "$$rest" | grep -q '\*' && echo " (active)" || echo ""); \
		printf "  %s%s\n" "$$NAME" "$$IS_ACTIVE"; \
	done; \
	echo ""

# ==============================================================================
# Go Build Commands
# ==============================================================================

# Install dependencies
deps:
	go mod download

# Install binary to ~/.local/bin
install: build
	mkdir -p ~/.local/bin
	rm -f ~/.local/bin/overlord
	cp overlord ~/.local/bin/overlord
	ln -sf overlord ~/.local/bin/ol
	@echo "Installed overlord to ~/.local/bin/overlord"
	@echo "Installed ol alias to ~/.local/bin/ol"

# Deploy to VPS (cross-compile for Linux and transfer)
# Usage: make deploy [VPS=hostname]
VPS ?= vps
deploy:
	@echo "Building for Linux amd64..."
	GOOS=linux GOARCH=amd64 go build -o overlord-linux ./cmd/overlord
	@echo "Deploying to $(VPS)..."
	scp overlord-linux $(VPS):/root/go/bin/overlord
	ssh $(VPS) "ln -sf overlord /root/go/bin/ol"
	rm overlord-linux
	@echo "Deployed to $(VPS):/root/go/bin/overlord"
	@echo "Deployed ol alias to $(VPS):/root/go/bin/ol"

# Build the application
build:
	go build -o overlord ./cmd/overlord

# Run the application
run:
	go run ./cmd/overlord

# ==============================================================================
# Overlord Commands
# ==============================================================================

# Set up overlord directories and configuration
setup:
	./overlord setup

# Open registry in editor
config:
	./overlord config

# Initialize directory with project template files
# Usage: make init [PATH=<path>] [LANG=go|py|ts|sol] [NAME=<name>] [DESC="description"]
init:
	./overlord init $(PATH) \
		$(if $(filter go,$(LANG)),--go) \
		$(if $(filter py,$(LANG)),--py) \
		$(if $(filter ts,$(LANG)),--ts) \
		$(if $(filter sol,$(LANG)),--sol) \
		$(if $(NAME),--name=$(NAME)) \
		$(if $(DESC),--description="$(DESC)")

# List projects (with optional filters)
# Usage: make list [CATEGORY=<cat>] [TAG=<tag>] [JSON=1] [ALL=1]
list:
	./overlord list $(if $(CATEGORY),--category=$(CATEGORY)) $(if $(TAG),--tag=$(TAG)) $(if $(JSON),--json) $(if $(ALL),--all)

# Show project info
info:
	./overlord info $(NAME)

# Detect unregistered projects on disk
# Usage: make detect [CATEGORY=<cat>] [JSON=1]
detect:
	./overlord detect $(if $(CATEGORY),--category=$(CATEGORY)) $(if $(JSON),--json)

# Create new project
# Usage: make new NAME=<name> CATEGORY=<cat> [LANG=go|py|ts|sol] [DESC="description"] [ALIAS=<alias>] [NO_GIT=1] [NO_OPEN=1]
new:
ifndef NAME
	$(error NAME is required. Usage: make new NAME=<name> CATEGORY=<cat> [LANG=go|py|ts|sol])
endif
ifndef CATEGORY
	$(error CATEGORY is required. Usage: make new NAME=<name> CATEGORY=<cat> [LANG=go|py|ts|sol])
endif
	./overlord new $(NAME) --category=$(CATEGORY) \
		$(if $(filter go,$(LANG)),--go) \
		$(if $(filter py,$(LANG)),--py) \
		$(if $(filter ts,$(LANG)),--ts) \
		$(if $(filter sol,$(LANG)),--sol) \
		$(if $(DESC),--description="$(DESC)") \
		$(if $(ALIAS),--alias=$(ALIAS)) \
		$(if $(NO_GIT),--no-git) \
		$(if $(NO_OPEN),--no-open)

# Add existing project to registry
# Usage: make add NAME=<name> PATH=<path> [CATEGORY=<cat>] [LANG=go|py|ts|sol] [DESC="description"] [ALIAS=<alias>]
add:
ifndef NAME
	$(error NAME is required. Usage: make add NAME=<name> PATH=<path> [CATEGORY=<cat>] [LANG=go|py|ts|sol])
endif
ifndef PATH
	$(error PATH is required. Usage: make add NAME=<name> PATH=<path> [CATEGORY=<cat>] [LANG=go|py|ts|sol])
endif
	./overlord add $(NAME) $(PATH) \
		$(if $(CATEGORY),--category=$(CATEGORY)) \
		$(if $(filter go,$(LANG)),--go) \
		$(if $(filter py,$(LANG)),--py) \
		$(if $(filter ts,$(LANG)),--ts) \
		$(if $(filter sol,$(LANG)),--sol) \
		$(if $(DESC),--description="$(DESC)") \
		$(if $(ALIAS),--alias=$(ALIAS))

# Remove project from registry
# Usage: make rm NAME=<name>
rm:
ifndef NAME
	$(error NAME is required. Usage: make rm NAME=<name>)
endif
	./overlord rm $(NAME)

# Clear project (remove from registry, delete files and thoughts)
# Usage: make clear NAME=<name>
clear:
ifndef NAME
	$(error NAME is required. Usage: make clear NAME=<name>)
endif
	./overlord clear $(NAME)

# Archive project to archive directory
# Usage: make archive NAME=<name> [YES=1]
archive:
ifndef NAME
	$(error NAME is required. Usage: make archive NAME=<name>)
endif
	./overlord archive $(NAME) $(if $(YES),--yes)

# Unarchive project from archive directory
# Usage: make unarchive NAME=<name> [CATEGORY=<cat>]
unarchive:
ifndef NAME
	$(error NAME is required. Usage: make unarchive NAME=<name> [CATEGORY=<cat>])
endif
	./overlord unarchive $(NAME) $(if $(CATEGORY),--category=$(CATEGORY))

# Open project workspace in tmux
# Usage: make open [NAME=<name>]
open:
	./overlord open $(NAME)

# Activate sync for a project
# Usage: make activate NAME=<name>
activate:
ifndef NAME
	$(error NAME is required. Usage: make activate NAME=<name>)
endif
	./overlord activate $(NAME)

# Deactivate sync for a project
# Usage: make deactivate NAME=<name>
deactivate:
ifndef NAME
	$(error NAME is required. Usage: make deactivate NAME=<name>)
endif
	./overlord deactivate $(NAME)

# Show Syncthing status for projects
# Usage: make sync [NAME=<name>]
sync:
	./overlord sync $(NAME)

# Run sync and config health checks
doctor:
	./overlord doctor

# Create one-way registry artifact backup
backup:
	./overlord backup

# Show current registry lock owner
sync-lock-show:
	./overlord sync lock show

# Transfer registry lock to another machine
# Usage: make sync-lock-transfer MACHINE=<name>
sync-lock-transfer:
ifndef MACHINE
	$(error MACHINE is required. Usage: make sync-lock-transfer MACHINE=<name>)
endif
	./overlord sync lock transfer $(MACHINE)

# Force-claim registry lock for local machine
sync-lock-claim:
	./overlord sync lock claim

# Pull .env file from pass for a project
# Usage: make env-pull [NAME=<name>]
env-pull:
	./overlord env pull $(NAME)

# Generate .env.pass template for a project
# Usage: make env-generate [NAME=<name>] [FORCE=1]
env-generate:
	./overlord env generate $(NAME) $(if $(FORCE),--force)

# ==============================================================================
# Worktree Commands (shortcuts for overlord worktree)
# ==============================================================================

# Create new worktree with tmux session
# Usage: make wt-new [BRANCH=<name>]
wt-new:
	./overlord worktree new $(BRANCH)

# List worktrees for current project
wt-list:
	./overlord worktree list

# Attach to worktree's tmux session
# Usage: make wt-attach BRANCH=<name>
wt-attach:
ifndef BRANCH
	$(error BRANCH is required. Usage: make wt-attach BRANCH=<name>)
endif
	./overlord worktree attach $(BRANCH)

# Remove worktree, branch, and tmux session
# Usage: make wt-remove BRANCH=<name>
wt-remove:
ifndef BRANCH
	$(error BRANCH is required. Usage: make wt-remove BRANCH=<name>)
endif
	./overlord worktree remove $(BRANCH)

# ==============================================================================
# Development Commands
# ==============================================================================

# Run tests
test:
	go test -v ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf bin/ dist/ coverage.out overlord
