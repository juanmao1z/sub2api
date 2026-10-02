.PHONY: build build-backend build-frontend

# Build the production application without test-only targets.
build: build-backend build-frontend

build-backend:
	@$(MAKE) -C backend build

build-frontend:
	@pnpm --dir frontend run build
