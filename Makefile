BINARY := cbeta-ebook-renamer
VERSION := $(shell grep -o 'version = "[^"]*"' cmd/cbeta-ebook-renamer/main.go | cut -d'"' -f2)
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
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/cbeta-ebook-renamer

# 建置全部平台並打包到 dist/。
# 執行檔與壓縮檔的命名規則與 .github/workflows/release.yml 相同，
# 差別只在本地的壓縮檔名多帶一段版本號（CI 的版本資訊由 tag 提供）。
#
# 執行檔一律叫 cbeta-ebook-renamer，平台資訊放在壓縮檔名上，
# 這樣 README 裡的指令對每個平台都是照抄就能用。macOS 版多一個 .command 副檔名：
# Finder 是看副檔名決定由誰開啟的，.command 會交給「終端機」執行；
# 若檔名裡含有點號(例如版本號 1.0.0)又沒有已知副檔名，LaunchServices 會判成
# dyn.* 未知類型，雙擊只會跳出「未設定任何應用程式來打開文件」。
#
# 一律打包壓縮檔，不放裸執行檔：瀏覽器從 GitHub Releases 下載裸檔會是 0644，
# 少了執行權限就無法雙擊；zip 與 tar 都會保留權限位元。
dist: test
	rm -rf dist && mkdir -p dist/stage
	@for p in $(PLATFORMS); do \
		os=$${p%%/*}; arch=$${p##*/}; ext=""; \
		case "$$os" in \
			windows) ext=".exe" ;; \
			darwin)  ext=".command" ;; \
		esac; \
		echo "building $$os/$$arch"; \
		rm -f dist/stage/*; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build -trimpath -ldflags="$(LDFLAGS)" -o dist/stage/$(BINARY)$$ext ./cmd/cbeta-ebook-renamer || exit 1; \
		cp README.md dist/stage/; \
		base=$(BINARY)-$(VERSION)-$$os-$$arch; \
		if [ "$$os" = "linux" ]; then \
			tar czf dist/$$base.tar.gz -C dist/stage $(BINARY)$$ext README.md || exit 1; \
		else \
			(cd dist/stage && zip -q -X ../$$base.zip $(BINARY)$$ext README.md) || exit 1; \
		fi; \
	done
	@rm -rf dist/stage
	@ls -lh dist/

clean:
	rm -rf dist $(BINARY) $(BINARY).exe
