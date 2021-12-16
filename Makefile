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

.PHONY: lint lint-ci fmt fmt-ci test test-ci clean release lint-docs

build:
	go build -ldflags "-X main.version=${VERSION} -X main.buildTimestamp=${BUILD_TIMESTAMP} -X main.revision=${REVISION}" -o gitlab-config gitlab.com/tozd/gitlab/config/cmd/gitlab-config

build-static:
	go build -ldflags "-linkmode external -extldflags '-static' -X main.version=${VERSION} -X main.buildTimestamp=${BUILD_TIMESTAMP} -X main.revision=${REVISION}" -o gitlab-config gitlab.com/tozd/gitlab/config/cmd/gitlab-config

lint:
	golangci-lint run --timeout 4m --color always

# TODO: Output both formats at the same time, once it is supported.
# See: https://github.com/golangci/golangci-lint/issues/481
lint-ci:
	-golangci-lint run --timeout 4m --color always
	golangci-lint run --timeout 4m --out-format code-climate > codeclimate.json

fmt:
	go mod tidy
	gofumpt -w *.go
	goimports -w -local gitlab.com/tozd/gitlab/config *.go

fmt-ci: fmt
	git diff --exit-code --color=always

test:
	gotestsum --format pkgname --packages ./... -- -race -timeout 10m -cover -covermode atomic

test-ci:
	gotestsum --format pkgname --packages ./... --junitfile tests.xml -- -race -timeout 10m -coverprofile=coverage.txt -covermode atomic
	gocover-cobertura < coverage.txt > coverage.xml
	go tool cover -html=coverage.txt -o coverage.html

clean:
	rm -f coverage.* codeclimate.json tests.xml

release:
	npx --yes --package 'release-it@14.11.6' --package 'git+https://github.com/mitar/keep-a-changelog.git#better-gitlab' -- release-it

lint-docs:
	npx --yes --package 'markdownlint-cli@~0.30.0' -- markdownlint --ignore-path .gitignore --ignore testdata/ '**/*.md'
