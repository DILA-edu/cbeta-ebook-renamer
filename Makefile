BINARY := cbeta-ebook-renamer
VERSION := $(shell grep -o 'version = "[^"]*"' main.go | cut -d'"' -f2)
LDFLAGS := -s -w
PLATFORMS := darwin/arm64 darwin/amd64 windows/amd64 windows/arm64 linux/amd64

.PHONY: all test vet build dist clean

all: test build

test:
	go test ./...

vet:
	go vet ./...
	GOOS=windows go vet ./...

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) .

# 建置全部平台到 dist/
dist: test
	rm -rf dist && mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%%/*}; arch=$${p##*/}; ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build -trimpath -ldflags="$(LDFLAGS)" \
			-o dist/$(BINARY)-$(VERSION)-$$os-$$arch$$ext . || exit 1; \
	done
	@ls -lh dist/

clean:
	rm -rf dist $(BINARY) $(BINARY).exe
