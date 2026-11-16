# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 專案概要

把 CBETA 電子書的名稱從「經號」批次改成「經號_經名」（例如 `T0001.epub` → `T0001_長阿含經.epub`），也能還原。
只用 Go 標準函式庫寫成的單一 CLI，主要使用者是**雙擊執行**的一般讀者，不是熟悉命令列的人。
程式輸出、註解、README、commit 訊息一律使用繁體中文。

## 常用指令

```bash
make test                     # go test ./...
make vet                      # go vet，並另跑 GOOS=windows 的 vet（console_windows.go 只有這樣才會被檢查）
make build                    # 建置目前平台的執行檔
make dist                     # 跑測試後建置全部平台並打包到 dist/
go test ./cmd/cbeta-ebook-renamer -run TestDocxRenamesFolderNotJuanFiles -v   # 只跑單一測試

go run ./cmd/cbeta-ebook-renamer -dry-run data/cbeta_epub_2026r2   # 用真實資料預覽，不會動檔案
```

`data/` 放的是 CBETA 官方下載檔與解壓結果（已在 .gitignore，不進版控），只供手動驗證；
測試一律用 `t.TempDir()` 自建假資料，不依賴 `data/`。對 `data/` 實際執行（不帶 `-dry-run`）會真的改名。

## 架構

所有程式碼都在 `cmd/cbeta-ebook-renamer/`（單一 `package main`）。執行流程：

1. `resolveRoot`：決定處理哪個資料夾（命令列參數 > 執行檔所在資料夾 > 工作目錄），判準是該層有沒有 `filelist_*.txt`。
2. `loadFileList`（filelist.go）：解析 CBETA 對照表（逗號分隔、UTF-8、CRLF、可能帶 BOM）。版本字串從檔名取得，不寫死。
3. `collectBooks`（plan.go）：掃出典籍項目。**epub/pdf 是一部典籍一個檔案；docx/odt 是一部典籍一個經號資料夾**，裡面按卷分成 `T0001_001.docx`…，此時改名對象是資料夾，卷檔維持原名、掃描也不會走進去。
4. `buildPlan` / `buildRestorePlan`：產生 `Plan`，每個 `Item` 標上一種 `Action`（改名、已完成、查無經號、無法辨識、卷檔、衝突）。
5. `runPlan`（main.go）：`checkExisting` → 預覽 → 確認 → `execute`（`os.Rename`）→ 寫 `rename_log.csv`（附加模式、帶 BOM）。

需要跨檔案理解的關鍵設計：

- **名稱分類（name.go 的 `parseBase`）**：主檔名分成純經號、已改名（`經號_經名`）、卷檔（`經號_數字`）、無法辨識。卷檔與已改名的樣式互相涵蓋，`reJuan` 的判斷必須排在 `reRenamed` 前面，否則會把卷號當成經名。
- **資料夾項目不可呼叫 `filepath.Ext`**：經名中可能有點號，`splitBookPath` 對資料夾一律回傳空副檔名。
- **絕不覆蓋**：衝突偵測用「資料夾 + 小寫目標名稱」當鍵（APFS/NTFS 不分大小寫）；`checkExisting` 另外檢查目標是否已被計畫外的檔案佔用。
- **可重複執行**：已是目標名稱的項目為 `ActSkipDone`；已改名但與目前對照表不符的會再更新並加註。
- **雙擊模式**：不帶任何參數（`len(os.Args) == 1`）視為雙擊，結束前 `pause()`；若正向計畫沒有可改的、而且整批都已改過名（`looksRenamed`），會改問要不要還原。所有互動輸入共用同一個 `bufio.Reader`（見 `stdin` 的註解）。
- **檔名清理**：`sanitizeTitle` 只替換非法字元、去頭尾空白/句點、截斷過長、避開 Windows 保留名稱；**刻意不做 Unicode 正規化**。
- **平台差異**：`console_windows.go` 把主控台切成 UTF-8 code page，其他平台是空函式（build tag 區分）。

## 版本與發佈

- 版本號是 `main.go` 的 `const version`，`Makefile` 用 grep 從這裡讀出版本，改版只需改這一處。
- 推送 `v*` tag 會觸發 `.github/workflows/release.yml`：vet（含 Windows）→ 測試 → 五個平台交叉編譯 → 建立 GitHub Release。
- 打包規則在 `Makefile` 的 `dist` 與 `release.yml` 兩處各寫一份，修改時兩邊要同步：
  - 執行檔一律叫 `cbeta-ebook-renamer`，平台資訊只放在壓縮檔名上（README 的指令才能照抄）。
  - macOS 版加 `.command` 副檔名，Finder 才會交給「終端機」執行。
  - 一律打包成 zip/tar.gz，不發裸執行檔（瀏覽器下載會失去執行權限）。
- Go 版本：`go.mod` 與 CI 都是 1.27，升級時兩邊一起改。
- 執行檔沒有程式碼簽章，README 有教使用者繞過 Gatekeeper/SmartScreen；改動相關流程時要一併更新 README。

## README 的角色

README 同時是終端使用者的說明書（會被打包進每個發佈壓縮檔），裡面記載了 2026r2 實測數字（4899 部、241 個重名、docx/odt 3687 部等）與各項設計理由。
改變行為、旗標或打包方式時，要同步更新 README 對應段落以及 `usage()` 與 `run()` 裡的中文說明文字。
