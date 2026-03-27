GOCACHE ?= $(CURDIR)/.cache/go-build
IMAGE ?= stash-scanner:dev
VERSION_FILE ?= VERSION
VERSION := $(shell cat $(VERSION_FILE))

.PHONY: fmt test run run-ui run-once docker-build clean docker-clean version check-version set-version tag release-patch release-minor release-major list-releases

fmt:
	GOCACHE=$(GOCACHE) gofmt -w ./cmd ./internal

test:
	GOCACHE=$(GOCACHE) go test ./...

run:
	GOCACHE=$(GOCACHE) go run ./cmd/scanner

run-ui:
	GOCACHE=$(GOCACHE) go run ./cmd/scanner

run-once:
	GOCACHE=$(GOCACHE) go run ./cmd/scanner -once

docker-build:
	docker build -t $(IMAGE) .

clean:
	rm -rf .cache data

docker-clean:
	docker image rm -f $(IMAGE) 2>/dev/null || true

version:
	@printf '%s\n' "$(VERSION)"

check-version:
	@if ! printf '%s' "$(VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "VERSION must be in semver format like 1.2.3"; \
		exit 1; \
	fi

set-version: check-version
	@printf '%s\n' "$(VERSION)" > $(VERSION_FILE)
	@echo "Updated $(VERSION_FILE) to $(VERSION)"

tag:
	@if [ -z "$(NEW_VERSION)" ]; then \
		echo "Usage: make tag NEW_VERSION=1.2.3"; \
		exit 1; \
	fi
	@if ! printf '%s' "$(NEW_VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "NEW_VERSION must be in semver format like 1.2.3"; \
		exit 1; \
	fi
	@printf '%s\n' "$(NEW_VERSION)" > $(VERSION_FILE)
	git add $(VERSION_FILE)
	git commit -m "chore(release): v$(NEW_VERSION)"
	git tag -a "v$(NEW_VERSION)" -m "Release v$(NEW_VERSION)"
	@echo "Created commit and tag for v$(NEW_VERSION)"
	@echo "Push with:"
	@echo "  git push origin main"
	@echo "  git push origin v$(NEW_VERSION)"

release-patch:
	@VERSION_PARTS=$$(printf '%s' "$(VERSION)" | awk -F. '{print $$1 " " $$2 " " $$3}'); \
	set -- $$VERSION_PARTS; \
	NEW_VERSION="$$1.$$2.$$(( $$3 + 1 ))"; \
	$(MAKE) tag NEW_VERSION=$$NEW_VERSION

release-minor:
	@VERSION_PARTS=$$(printf '%s' "$(VERSION)" | awk -F. '{print $$1 " " $$2 " " $$3}'); \
	set -- $$VERSION_PARTS; \
	NEW_VERSION="$$1.$$(( $$2 + 1 )).0"; \
	$(MAKE) tag NEW_VERSION=$$NEW_VERSION

release-major:
	@VERSION_PARTS=$$(printf '%s' "$(VERSION)" | awk -F. '{print $$1 " " $$2 " " $$3}'); \
	set -- $$VERSION_PARTS; \
	NEW_VERSION="$$(( $$1 + 1 )).0.0"; \
	$(MAKE) tag NEW_VERSION=$$NEW_VERSION

list-releases:
	@git tag -l "v*.*.*" | sort -V | tail -10
