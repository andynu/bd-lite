.PHONY: build test fmt fmt-check

# beads/ is a vendored copy of upstream and is not gofmt-clean; formatting it
# would create noise in every diff against upstream.
OWN_GO_FILES = $(shell git ls-files '*.go' | grep -v '^beads/')

build:
	go build ./...

test:
	go test ./...

# Rewrite our own Go files in place.
fmt:
	@gofmt -w $(OWN_GO_FILES)

# Fail if any of our own Go files need gofmt. Run before committing.
fmt-check:
	@unformatted=$$(gofmt -l . | grep -v '^beads/' || true); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt-clean:"; \
		echo "$$unformatted" | sed 's/^/  /'; \
		echo; \
		echo "fix with: make fmt"; \
		exit 1; \
	fi
	@echo "gofmt: clean"
