// cbeta-ebook-renamer：把 CBETA 電子書的名稱從「經號」改成「經號_經名」。
//
// epub 與 pdf 是一部典籍一個檔案，改的是檔名；
// docx 與 odt 是一部典籍一個資料夾、裡面再按卷分檔，改的是資料夾名，卷檔維持原名。
//
// 用法最簡單的形式是把執行檔放進解壓後的 cbeta_epub_2026r2 資料夾裡直接執行，
// 程式會自動找到同一層的 filelist_*.txt 對照表。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const version = "1.2.0"

var (
	flagDryRun  = flag.Bool("dry-run", false, "只顯示預覽，不實際改名")
	flagYes     = flag.Bool("yes", false, "略過確認直接執行（供批次腳本使用）")
	flagRestore = flag.Bool("restore", false, "還原：把「經號_經名」改回「經號」")
	flagLog     = flag.String("log", "rename_log.csv", "改名紀錄檔路徑，設為空字串則不產生")
	flagList    = flag.Int("list", 20, "預覽時最多列出幾筆，設為 0 表示全部列出")
	flagExt     = flag.String("ext", ".epub,.pdf,.docx,.odt", "要處理的副檔名，以逗號分隔")
	flagVersion = flag.Bool("version", false, "顯示版本後結束")
)

// 標準錯誤輸出的警告訊息
func warnf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "  ! "+format+"\n", a...)
}

func main() {
	initConsole()
	flag.Usage = usage
	flag.Parse()

	if *flagVersion {
		fmt.Printf("cbeta-ebook-renamer %s\n", version)
		return
	}

	// 沒有帶任何參數時視為「雙擊執行」，結束前會停住等使用者按鍵。
	interactive := len(os.Args) == 1

	code := run(interactive)
	if interactive {
		pause()
	}
	os.Exit(code)
}

func usage() {
	fmt.Fprintf(os.Stderr, `cbeta-ebook-renamer %s — CBETA 電子書名稱批次轉換工具

用法：
  cbeta-ebook-renamer [選項] [CBETA 電子書資料夾路徑]

不指定資料夾時，依序嘗試「執行檔所在資料夾」與「目前工作目錄」。
不帶任何選項直接執行（例如雙擊）時，若資料夾裡的名稱都已經加過經名，
會改問要不要還原成只有經號，不必自己補上 -restore。

選項：
`, version)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
範例：
  cbeta-ebook-renamer                       # 放在 epub 資料夾裡直接執行
  cbeta-ebook-renamer -dry-run ~/Downloads/cbeta_epub_2026r2
  cbeta-ebook-renamer -yes ~/Downloads/cbeta_epub_2026r2
  cbeta-ebook-renamer -restore ~/Downloads/cbeta_epub_2026r2
  cbeta-ebook-renamer -dry-run ~/Downloads/docx      # docx/odt 改的是經號資料夾
`)
}

func run(interactive bool) int {
	fmt.Printf("cbeta-ebook-renamer %s — CBETA 電子書名稱批次轉換工具\n\n", version)

	root, err := resolveRoot(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：%v\n\n", err)
		fmt.Fprintln(os.Stderr, "請把本程式放進解壓後的電子書資料夾中執行（epub 是 cbeta_epub_XXXX、")
		fmt.Fprintln(os.Stderr, "pdf 是 cbeta_pdf_X_XXXX、docx 與 odt 則是含有 T/、X/ 的那一層），")
		fmt.Fprintln(os.Stderr, "或是在命令列指定該資料夾的路徑。")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "註：docx 與 odt 的下載檔目前不含 filelist_*.txt 對照表，")
		fmt.Fprintln(os.Stderr, "請先從 epub 或 pdf 版的壓縮檔中複製一份過來。")
		return 1
	}

	flPath, err := findFileList(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：%v\n", err)
		return 1
	}
	fl, err := loadFileList(flPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：讀取對照表失敗：%v\n", err)
		return 1
	}

	fmt.Printf("資料夾　：%s\n", root)
	fmt.Printf("對照表　：%s（%d 筆）\n", filepath.Base(flPath), len(fl.Entries))
	checkVersion(root, fl)

	exts := parseExts(*flagExt)
	if len(exts) == 0 {
		fmt.Fprintln(os.Stderr, "錯誤：-ext 至少要指定一種副檔名。")
		return 1
	}
	files, err := collectBooks(root, exts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "錯誤：掃描檔案失敗：%v\n", err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "\n錯誤：這個資料夾裡找不到任何 %s 檔案。\n", strings.Join(exts, "、"))
		return 1
	}
	dirCount := 0
	for _, bp := range files {
		if bp.IsDir {
			dirCount++
		}
	}
	if dirCount > 0 {
		fmt.Printf("典籍項目：%d 個（其中 %d 個是按卷分檔的經號資料夾）\n", len(files), dirCount)
	} else {
		fmt.Printf("典籍檔案：%d 個（%s）\n", len(files), strings.Join(exts, "、"))
	}
	// PDF 版是分三次下載的，每一份都帶完整的對照表，
	// 所以「檔案數比對照表少」是正常現象，只有多出來才值得警告。
	if len(files) > len(fl.Entries) {
		warnf("檔案數(%d)超過對照表筆數(%d)，請確認資料夾裡沒有混入其他來源的檔案",
			len(files), len(fl.Entries))
	}

	if *flagRestore {
		code, nothing := runPlan(root, buildRestorePlan(root, fl, files), "還原")
		if nothing {
			fmt.Println("\n沒有需要處理的檔案，結束。")
		}
		return code
	}

	plan := buildPlan(root, fl, files)
	code, nothing := runPlan(root, plan, "改名")
	if !nothing {
		return code
	}

	// 正向沒有任何可改的項目時，若整批檔名都已經加過經名，
	// 雙擊執行的使用者多半是想還原——他們沒有辦法自己補上 -restore。
	if !interactive || !looksRenamed(plan.Counts()) {
		fmt.Println("\n沒有需要處理的檔案，結束。")
		return code
	}
	fmt.Println("\n這個資料夾的檔名看起來已經加上經名了。")
	if !confirm("要還原成只有經號的檔名嗎？") {
		fmt.Println("已取消，未更動任何檔案。")
		return 0
	}
	code, nothing = runPlan(root, buildRestorePlan(root, fl, files), "還原")
	if nothing {
		fmt.Println("\n沒有需要處理的檔案，結束。")
	}
	return code
}

// looksRenamed 判斷這個資料夾是不是「已經整批改過名」的狀態：
// 正向計畫裡沒有任何待改名的項目，卻有一批項目是「已經是目標檔名」。
// 部分改過、部分沒改的混合狀態不算，那種情況照舊直接走正向改名。
func looksRenamed(c map[Action]int) bool {
	return c[ActRename] == 0 && c[ActSkipDone] > 0
}

// runPlan 把一份計畫跑完：印預覽、統計、確認、實際執行。
// nothing 為 true 表示這份計畫沒有任何待處理的項目，
// 結束訊息交由呼叫端決定，因為呼叫端可能還想改問別的。
func runPlan(root string, plan *Plan, op string) (code int, nothing bool) {
	plan.checkExisting()

	fmt.Printf("\n── %s預覽 ─────────────────────────\n", op)
	printPreview(plan)

	c := plan.Counts()
	fmt.Printf("\n待%s：%d　略過：%d　衝突：%d\n",
		op, c[ActRename],
		c[ActSkipDone]+c[ActSkipNoEntry]+c[ActSkipUnknown]+c[ActSkipJuan], c[ActConflict])
	if c[ActSkipNoEntry] > 0 {
		warnf("有 %d 個檔案的經號不在對照表中，已略過", c[ActSkipNoEntry])
	}
	if c[ActSkipJuan] > 0 {
		warnf("有 %d 個按卷分檔（如 T0001_001）不在任何經號資料夾底下，已略過", c[ActSkipJuan])
	}

	if c[ActConflict] > 0 {
		fmt.Println("\n衝突的項目會原樣保留，不會被覆蓋：")
		n := 0
		for _, it := range plan.Items {
			if it.Action != ActConflict {
				continue
			}
			n++
			if *flagList > 0 && n > *flagList {
				fmt.Printf("  …另有 %d 筆\n", c[ActConflict]-*flagList)
				break
			}
			fmt.Printf("  %s/%s → %s（%s）\n", it.Dir, it.DisplayOld(), it.DisplayNew(), it.Note)
		}
	}
	plan.warnLongPaths()

	if c[ActRename] == 0 {
		return 0, true
	}
	if *flagDryRun {
		fmt.Println("\n(-dry-run 模式，未實際更動任何檔案)")
		return 0, false
	}
	if !*flagYes && !confirm(fmt.Sprintf("\n確定要%s這 %d 個項目嗎？", op, c[ActRename])) {
		fmt.Println("已取消，未更動任何檔案。")
		return 0, false
	}

	logPath := *flagLog
	if logPath != "" && !filepath.IsAbs(logPath) {
		logPath = filepath.Join(root, logPath)
	}
	logw, err := newLogWriter(logPath, op)
	if err != nil {
		warnf("無法建立紀錄檔 %s：%v（改名仍會繼續）", logPath, err)
		logw, _ = newLogWriter("", op)
	}
	defer logw.close()

	fmt.Printf("\n處理中…\n")
	done, errs := plan.execute(logw)
	logw.close()

	fmt.Printf("\n完成：%s %d 個項目。\n", op, done)
	if logPath != "" {
		fmt.Printf("紀錄已寫入：%s\n", logPath)
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "\n有 %d 個項目處理失敗：\n", len(errs))
		for i, e := range errs {
			if i >= 10 {
				fmt.Fprintf(os.Stderr, "  …另有 %d 筆\n", len(errs)-10)
				break
			}
			fmt.Fprintf(os.Stderr, "  %v\n", e)
		}
		return 1, false
	}
	return 0, false
}

// resolveRoot 決定要處理哪個資料夾。
// 優先序：命令列參數 > 執行檔所在資料夾 > 目前工作目錄。
func resolveRoot(arg string) (string, error) {
	var candidates []string
	if arg != "" {
		abs, err := filepath.Abs(arg)
		if err != nil {
			return "", err
		}
		if !hasFileList(abs) {
			return "", fmt.Errorf("%s 裡面沒有 filelist_*.txt 對照表", abs)
		}
		return abs, nil
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		candidates = append(candidates, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	for _, c := range candidates {
		if hasFileList(c) {
			return c, nil
		}
	}
	return "", fmt.Errorf("找不到含有 filelist_*.txt 的資料夾")
}

// parseExts 把 "-ext .epub,.pdf" 轉成正規化後的副檔名清單。
func parseExts(s string) []string {
	var out []string
	for _, e := range strings.Split(s, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		out = append(out, e)
	}
	return out
}

func hasFileList(dir string) bool {
	m, err := filepath.Glob(filepath.Join(dir, "filelist_*.txt"))
	return err == nil && len(m) > 0
}

// cbeta_epub_2026r2 → 2026r2、cbeta_pdf_1_2026r2 → 2026r2
// 中間的 _1 是 PDF 分三次下載的批次編號。
var reDirVersion = regexp.MustCompile(`^cbeta_[a-z]+(?:_[0-9]+)?_(.+)$`)

// checkVersion 比對資料夾名稱與對照表檔名中的版本字串是否一致。
func checkVersion(root string, fl *FileList) {
	m := reDirVersion.FindStringSubmatch(filepath.Base(root))
	if m == nil || fl.Version == "" {
		return
	}
	if m[1] != fl.Version {
		warnf("資料夾版本(%s)與對照表版本(%s)不符，請確認兩者來自同一次下載", m[1], fl.Version)
	}
}

func printPreview(plan *Plan) {
	n := 0
	total := plan.Counts()[ActRename]
	for _, it := range plan.Items {
		if it.Action != ActRename {
			continue
		}
		n++
		if *flagList > 0 && n > *flagList {
			fmt.Printf("  …另有 %d 筆（用 -list 0 可列出全部）\n", total-*flagList)
			break
		}
		dir := it.Dir
		if dir != "" {
			dir += string(filepath.Separator)
		}
		line := fmt.Sprintf("  %s%s → %s", dir, it.DisplayOld(), it.DisplayNew())
		if it.Note != "" {
			line += "  [" + it.Note + "]"
		}
		fmt.Println(line)
	}
	if total == 0 {
		fmt.Println("  （無）")
	}
}

// 所有互動輸入共用同一個 reader。bufio 會預讀一整塊，
// 每問一個問題就新建一個 reader 的話，一次執行中問到第二個問題時，
// 答案早已被前一個 reader 讀進緩衝區丟掉了。
var stdin = bufio.NewReader(os.Stdin)

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	line, err := stdin.ReadString('\n')
	if line == "" && err != nil {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes"
}

func pause() {
	fmt.Print("\n按 Enter 鍵結束…")
	stdin.ReadString('\n')
}
