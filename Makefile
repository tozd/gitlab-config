SHELL = /bin/bash -o pipefail

# We use ifeq instead of ?= so that we set variables
# also when they are defined, but empty.
ifeq ($(VERSION),)
 VERSION = `git describe --tags --always --dirty=+`
endif
ifeq ($(BUILD_TIMESTAMP),)
 BUILD_TIMESTAMP = `date -u +%FT%TZ`
endif
ifeq ($(REVISION),)
 REVISION = `git rev-parse HEAD`
endif

.PHONY: build build-static test test-ci lint lint-ci fmt fmt-ci clean release lint-docs audit encrypt decrypt sops

build:
	go build -trimpath -ldflags "-s -w -X main.version=${VERSION} -X main.buildTimestamp=${BUILD_TIMESTAMP} -X main.revision=${REVISION}" -o gitlab-config gitlab.com/tozd/gitlab/config/cmd/gitlab-config

build-static:
	go build -trimpath -ldflags "-s -w -linkmode external -extldflags '-static' -X main.version=${VERSION} -X main.buildTimestamp=${BUILD_TIMESTAMP} -X main.revision=${REVISION}" -o gitlab-config gitlab.com/tozd/gitlab/config/cmd/gitlab-config

test:
	gotestsum --format pkgname --packages ./... -- -race -timeout 10m -cover -covermode atomic

test-ci:
	gotestsum --format pkgname --packages ./... --junitfile tests.xml -- -race -timeout 10m -coverprofile=coverage.txt -covermode atomic
	gocover-cobertura < coverage.txt > coverage.xml
	go tool cover -html=coverage.txt -o coverage.html

lint:
	golangci-lint run --timeout 4m --color always --allow-parallel-runners --fix

lint-ci:
	golangci-lint run --timeout 4m --out-format colored-line-number,code-climate:codeclimate.json

fmt:
	go mod tidy
	git ls-files --cached --modified --other --exclude-standard -z | grep -z -Z '.go$$' | xargs -0 gofumpt -w
	git ls-files --cached --modified --other --exclude-standard -z | grep -z -Z '.go$$' | xargs -0 goimports -w -local gitlab.com/tozd/gitlab/config

fmt-ci: fmt
	git diff --exit-code --color=always

clean:
	rm -f coverage.* codeclimate.json tests.xml gitlab-config

release:
	npx --yes --package 'release-it@15.4.2' --package '@release-it/keep-a-changelog@3.1.0' -- release-it

lint-docs:
	npx --yes --package 'markdownlint-cli@~0.34.0' -- markdownlint --ignore-path .gitignore --ignore testdata/ '**/*.md'

audit:
	go list -json -deps ./... | nancy sleuth --skip-update-check

encrypt: build
	./gitlab-config sops --encrypt --mac-only-encrypted --in-place --encrypted-comment-regex sops:enc .gitlab-conf.yml

decrypt: build
	SOPS_AGE_KEY_FILE=keys.txt ./gitlab-config sops --decrypt --in-place .gitlab-conf.yml

sops: build
	SOPS_AGE_KEY_FILE=keys.txt ./gitlab-config sops .gitlab-conf.yml
