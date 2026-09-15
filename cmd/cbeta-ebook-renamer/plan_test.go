package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func testFileList() *FileList {
	fl := &FileList{Version: "test", Entries: map[string]Entry{}}
	add := func(id, title string) {
		fl.Entries[id] = Entry{ID: id, Title: title}
		fl.Order = append(fl.Order, id)
	}
	add("T0001", "長阿含經")
	add("T0374", "大般涅槃經")
	add("T0375", "大般涅槃經") // 同名不同經：這正是必須保留經號前綴的原因
	add("T0376", "大般涅槃經")
	return fl
}

// bookFiles 把測試用的相對路徑清單轉成掃描結果（皆為單一檔案）。
func bookFiles(rels ...string) []bookPath {
	out := make([]bookPath, 0, len(rels))
	for _, r := range rels {
		out = append(out, bookPath{Rel: filepath.FromSlash(r)})
	}
	return out
}

// bookDirs 同上，但每一筆都是 docx/odt 的經號資料夾。
func bookDirs(rels ...string) []bookPath {
	out := make([]bookPath, 0, len(rels))
	for _, r := range rels {
		out = append(out, bookPath{Rel: filepath.FromSlash(r), IsDir: true})
	}
	return out
}

func TestBuildPlanKeepsDuplicateTitlesApart(t *testing.T) {
	fl := testFileList()
	files := bookFiles("T/T0001.epub", "T/T0374.epub", "T/T0375.epub", "T/T0376.epub")
	p := buildPlan("/root", fl, files)

	if n := p.Counts()[ActRename]; n != 4 {
		t.Fatalf("預期 4 筆改名，得到 %d", n)
	}
	seen := map[string]bool{}
	for _, it := range p.Items {
		if seen[it.NewName] {
			t.Errorf("目標檔名重複：%s", it.NewName)
		}
		seen[it.NewName] = true
	}
	if !seen["T0374_大般涅槃經.epub"] || !seen["T0375_大般涅槃經.epub"] {
		t.Error("同名經典應該靠經號前綴區分開來")
	}
}

func TestBuildPlanIdempotent(t *testing.T) {
	fl := testFileList()
	// 已經改過名的檔案再跑一次，應該全部略過
	files := bookFiles("T/T0001_長阿含經.epub")
	p := buildPlan("/root", fl, files)
	if p.Items[0].Action != ActSkipDone {
		t.Errorf("已改名的檔案應該略過，得到 %v", p.Items[0].Action)
	}
}

func TestBuildPlanUnknownID(t *testing.T) {
	fl := testFileList()
	p := buildPlan("/root", fl, bookFiles("T/T9999.epub", "T/readme.epub"))
	if p.Items[0].Action != ActSkipNoEntry {
		t.Errorf("對照表查無的經號應標記為 ActSkipNoEntry，得到 %v", p.Items[0].Action)
	}
	if p.Items[1].Action != ActSkipUnknown {
		t.Errorf("無法辨識的檔名應標記為 ActSkipUnknown，得到 %v", p.Items[1].Action)
	}
}

func TestRestorePlan(t *testing.T) {
	fl := testFileList()
	files := bookFiles("T/T0001_長阿含經.epub", "T/T0374.epub")
	p := buildRestorePlan("/root", fl, files)
	if p.Items[0].Action != ActRename || p.Items[0].NewName != "T0001.epub" {
		t.Errorf("還原結果不正確：%+v", p.Items[0])
	}
	if p.Items[1].Action != ActSkipDone {
		t.Errorf("原本就是經號檔名的應略過，得到 %v", p.Items[1].Action)
	}
}

// PDF 與 EPUB 形狀相同，同一套邏輯要能通吃。
func TestBuildPlanMixedExtensions(t *testing.T) {
	fl := testFileList()
	p := buildPlan("/root", fl, bookFiles("T/T0001.epub", "T/T0001.pdf"))
	want := map[string]bool{"T0001_長阿含經.epub": true, "T0001_長阿含經.pdf": true}
	for _, it := range p.Items {
		if it.Action != ActRename {
			t.Fatalf("%s 應該要改名，得到 %v", it.OldName, it.Action)
		}
		if !want[it.NewName] {
			t.Errorf("非預期的目標檔名：%s", it.NewName)
		}
		delete(want, it.NewName)
	}
	if len(want) != 0 {
		t.Errorf("以下目標檔名沒有產生：%v", want)
	}
}

// docx/odt 的按卷分檔絕對不能被當成「已改名」處理。
// 若誤判，改名會讓 22 卷全部指向同一個檔名，還原則會把卷號整個砍掉。
func TestJuanFilesAreNeverTouched(t *testing.T) {
	fl := testFileList()
	files := bookFiles("T/T0001/T0001_001.docx", "T/T0001/T0001_002.docx")

	for _, tc := range []struct {
		name string
		p    *Plan
	}{
		{"改名", buildPlan("/root", fl, files)},
		{"還原", buildRestorePlan("/root", fl, files)},
	} {
		for _, it := range tc.p.Items {
			if it.Action != ActSkipJuan {
				t.Errorf("%s模式：%s 應標記為 ActSkipJuan，得到 %v（NewName=%q）",
					tc.name, it.OldName, it.Action, it.NewName)
			}
			if it.NewName != "" {
				t.Errorf("%s模式：%s 不應產生目標檔名，卻得到 %q", tc.name, it.OldName, it.NewName)
			}
		}
	}
}

// 還原模式不可以動到對照表中沒有的檔案（例如使用者自己放進來的書）。
func TestRestorePlanSkipsUnknownID(t *testing.T) {
	fl := testFileList()
	p := buildRestorePlan("/root", fl, bookFiles("T/T9999_我自己的書.epub"))
	if p.Items[0].Action != ActSkipNoEntry {
		t.Errorf("對照表查無的經號不應還原，得到 %v", p.Items[0].Action)
	}
}

// 完整走一次實際檔案的改名與還原，確認可逆。
func TestExecuteRoundTrip(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "T")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"T0001", "T0374", "T0375"} {
		if err := os.WriteFile(filepath.Join(dir, id+".epub"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fl := testFileList()
	logw, _ := newLogWriter("", "改名")

	files, err := collectBooks(root, []string{".epub", ".pdf"})
	if err != nil {
		t.Fatal(err)
	}
	p := buildPlan(root, fl, files)
	p.checkExisting()
	done, errs := p.execute(logw)
	if len(errs) > 0 || done != 3 {
		t.Fatalf("改名失敗：done=%d errs=%v", done, errs)
	}
	got := lsNames(t, dir)
	want := []string{"T0001_長阿含經.epub", "T0374_大般涅槃經.epub", "T0375_大般涅槃經.epub"}
	if !equal(got, want) {
		t.Fatalf("改名後檔名 = %v，預期 %v", got, want)
	}

	// 再跑一次，應該完全不動作（冪等）
	files, _ = collectBooks(root, []string{".epub", ".pdf"})
	p2 := buildPlan(root, fl, files)
	p2.checkExisting()
	if n := p2.Counts()[ActRename]; n != 0 {
		t.Errorf("重複執行不應再改名，卻有 %d 筆", n)
	}

	// 還原
	files, _ = collectBooks(root, []string{".epub", ".pdf"})
	p3 := buildRestorePlan(root, fl, files)
	p3.checkExisting()
	done, errs = p3.execute(logw)
	if len(errs) > 0 || done != 3 {
		t.Fatalf("還原失敗：done=%d errs=%v", done, errs)
	}
	got = lsNames(t, dir)
	want = []string{"T0001.epub", "T0374.epub", "T0375.epub"}
	if !equal(got, want) {
		t.Fatalf("還原後檔名 = %v，預期 %v", got, want)
	}
}

// 目標檔名已被別的檔案佔用時，必須標記為衝突而不是覆蓋掉。
func TestCheckExistingBlocksOverwrite(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "T")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "T0001.epub"), []byte("new"), 0o644)
	os.WriteFile(filepath.Join(dir, "T0001_長阿含經.epub"), []byte("old"), 0o644)

	fl := testFileList()
	files, _ := collectBooks(root, []string{".epub", ".pdf"})
	p := buildPlan(root, fl, files)
	p.checkExisting()
	for _, it := range p.Items {
		if it.OldName == "T0001.epub" && it.Action != ActConflict {
			t.Errorf("目標已存在時應標記衝突，得到 %v", it.Action)
		}
	}
	logw, _ := newLogWriter("", "改名")
	p.execute(logw)
	b, _ := os.ReadFile(filepath.Join(dir, "T0001_長阿含經.epub"))
	if string(b) != "old" {
		t.Error("既有檔案被覆蓋了")
	}
}

func TestLoadFileList(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "filelist_2026r2.txt")
	content := "\ufeff經號 , 冊數 , 卷數 , 經名 , 作譯者\r\n" +
		"=======================================================\r\n" +
		"T0001 , 01 , 22 , 長阿含經 , 後秦 佛陀耶舍共竺佛念譯\r\n" +
		"T1159A , 20 , 1 , [峚-大+(企-止)]𡇪大道心驅策法 , 失譯\r\n"
	os.WriteFile(path, []byte(content), 0o644)

	fl, err := loadFileList(path)
	if err != nil {
		t.Fatal(err)
	}
	if fl.Version != "2026r2" {
		t.Errorf("版本解析錯誤：%q", fl.Version)
	}
	if len(fl.Entries) != 2 {
		t.Fatalf("預期 2 筆，得到 %d", len(fl.Entries))
	}
	if fl.Entries["T0001"].Title != "長阿含經" {
		t.Errorf("經名解析錯誤：%q", fl.Entries["T0001"].Title)
	}
	if fl.Entries["T0001"].Author != "後秦 佛陀耶舍共竺佛念譯" {
		t.Errorf("作譯者解析錯誤：%q", fl.Entries["T0001"].Author)
	}
}

func lsNames(t *testing.T, dir string) []string {
	t.Helper()
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, d := range des {
		names = append(names, d.Name())
	}
	sort.Strings(names)
	return names
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 紀錄檔應該累加而不是每次覆蓋，否則還原會洗掉改名的對照紀錄。
func TestLogWriterAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rename_log.csv")
	it := Item{Dir: "T", OldName: "T0001.epub", NewName: "T0001_長阿含經.epub"}

	w1, err := newLogWriter(path, "改名")
	if err != nil {
		t.Fatal(err)
	}
	w1.write(it, "成功")
	w1.close()

	w2, err := newLogWriter(path, "還原")
	if err != nil {
		t.Fatal(err)
	}
	w2.write(Item{Dir: "T", OldName: it.NewName, NewName: it.OldName}, "成功")
	w2.close()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	if !strings.Contains(content, "改名") || !strings.Contains(content, "還原") {
		t.Errorf("兩次執行的紀錄都應保留，實際內容：\n%s", content)
	}
	if n := strings.Count(content, "時間,操作"); n != 1 {
		t.Errorf("標題列應該只有一行，實際有 %d 行", n)
	}
	if n := strings.Count(content, "\ufeff"); n != 1 {
		t.Errorf("BOM 應該只出現在檔首一次，實際有 %d 個", n)
	}
}

// ── docx / odt：改名對象是經號資料夾，不是裡面的卷檔 ─────────────

// makeDocxTree 造出 T/T0001/T0001_001.docx 這種按卷分檔的樹狀結構。
func makeDocxTree(t *testing.T, root string, ext string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		dir := filepath.Join(root, "T", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, juan := range []string{"001", "002", "003"} {
			f := filepath.Join(dir, id+"_"+juan+ext)
			if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// 掃描 docx 樹時，每部典籍只該產生「一筆資料夾項目」，
// 而不是把裡面的每一卷都列出來。
func TestCollectBooksFindsBookDirs(t *testing.T) {
	root := t.TempDir()
	makeDocxTree(t, root, ".docx", "T0001", "T0374")

	files, err := collectBooks(root, []string{".docx", ".odt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("預期 2 筆典籍資料夾，得到 %d 筆：%+v", len(files), files)
	}
	for _, bp := range files {
		if !bp.IsDir {
			t.Errorf("%s 應標記為資料夾", bp.Rel)
		}
	}
}

// 資料夾整體改名，卷檔一律不動。
func TestDocxRenamesFolderNotJuanFiles(t *testing.T) {
	root := t.TempDir()
	makeDocxTree(t, root, ".docx", "T0001", "T0374", "T0375")
	fl := testFileList()
	logw, _ := newLogWriter("", "改名")

	files, _ := collectBooks(root, []string{".docx"})
	p := buildPlan(root, fl, files)
	p.checkExisting()
	if n := p.Counts()[ActSkipJuan]; n != 0 {
		t.Errorf("卷檔不該被逐一走訪，卻有 %d 筆按卷分檔", n)
	}
	done, errs := p.execute(logw)
	if len(errs) > 0 || done != 3 {
		t.Fatalf("改名失敗：done=%d errs=%v", done, errs)
	}

	got := lsNames(t, filepath.Join(root, "T"))
	want := []string{"T0001_長阿含經", "T0374_大般涅槃經", "T0375_大般涅槃經"}
	if !equal(got, want) {
		t.Fatalf("改名後資料夾 = %v，預期 %v", got, want)
	}
	// 卷檔必須原封不動
	inner := lsNames(t, filepath.Join(root, "T", "T0001_長阿含經"))
	wantInner := []string{"T0001_001.docx", "T0001_002.docx", "T0001_003.docx"}
	if !equal(inner, wantInner) {
		t.Errorf("卷檔被動到了：%v，預期 %v", inner, wantInner)
	}

	// 再跑一次應該完全不動作（冪等）
	files, _ = collectBooks(root, []string{".docx"})
	p2 := buildPlan(root, fl, files)
	p2.checkExisting()
	if n := p2.Counts()[ActRename]; n != 0 {
		t.Errorf("重複執行不應再改名，卻有 %d 筆", n)
	}
	if n := p2.Counts()[ActSkipDone]; n != 3 {
		t.Errorf("預期 3 筆「已改名」，得到 %d", n)
	}

	// 還原
	files, _ = collectBooks(root, []string{".docx"})
	p3 := buildRestorePlan(root, fl, files)
	p3.checkExisting()
	done, errs = p3.execute(logw)
	if len(errs) > 0 || done != 3 {
		t.Fatalf("還原失敗：done=%d errs=%v", done, errs)
	}
	got = lsNames(t, filepath.Join(root, "T"))
	want = []string{"T0001", "T0374", "T0375"}
	if !equal(got, want) {
		t.Fatalf("還原後資料夾 = %v，預期 %v", got, want)
	}
}

// 資料夾名稱絕不能被當成「主檔名＋副檔名」拆開，
// 否則經名中若出現點號，尾巴會被當副檔名切掉。
func TestBookDirNameIsNeverSplitByExtension(t *testing.T) {
	fl := &FileList{Entries: map[string]Entry{
		"T0001": {ID: "T0001", Title: "長阿含經.卷上"},
	}}
	p := buildPlan("/root", fl, bookDirs("T/T0001"))
	if got := p.Items[0].NewName; got != "T0001_長阿含經.卷上" {
		t.Errorf("資料夾目標名稱 = %q，預期 T0001_長阿含經.卷上", got)
	}
	// 反向：已改名的資料夾還原時也不可以把「.卷上」當副檔名留下來
	r := buildRestorePlan("/root", fl, bookDirs("T/T0001_長阿含經.卷上"))
	if got := r.Items[0].NewName; got != "T0001" {
		t.Errorf("還原目標名稱 = %q，預期 T0001", got)
	}
}

// odt 與 docx 形狀相同，同一套邏輯要能通吃。
func TestOdtBookDirs(t *testing.T) {
	root := t.TempDir()
	makeDocxTree(t, root, ".odt", "T0001")
	files, _ := collectBooks(root, []string{".epub", ".pdf", ".docx", ".odt"})
	if len(files) != 1 || !files[0].IsDir {
		t.Fatalf("odt 的經號資料夾未被辨識：%+v", files)
	}
}

// 不在任何經號資料夾底下的孤立卷檔仍要明確略過，不能誤判成已改名。
func TestLooseJuanFileStillSkipped(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "T")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "T0001_001.docx"), []byte("x"), 0o644)

	files, _ := collectBooks(root, []string{".docx"})
	p := buildPlan(root, testFileList(), files)
	if len(p.Items) != 1 || p.Items[0].Action != ActSkipJuan {
		t.Fatalf("孤立卷檔應標記為 ActSkipJuan：%+v", p.Items)
	}
}

// docx/odt 內附的對照表由 CBETA 的書目資料庫自產，與 epub 版有兩點不同：
// 沒有 BOM、檔尾不換行，而且只收錄該壓縮檔實際包含的經號（子集）。
// 三者都必須能正常解析。
func TestLoadFileListNoBOMNoTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filelist_2026r2.txt")
	content := "經號 , 冊數 , 卷數 , 經名 , 作譯者\r\n" +
		strings.Repeat("=", 55) + "\r\n" +
		"T0001 , 01 , 22 , 長阿含經 , 後秦 佛陀耶舍共竺佛念譯\r\n" +
		"T1567 , 30 , 9 , 大乘中觀釋論 , 安慧菩薩造　宋 惟淨等譯\r\n" +
		"X1190 , 62 , 1 , 念佛三昧 , " // 末欄可為空，且檔尾不換行
	os.WriteFile(path, []byte(content), 0o644)

	fl, err := loadFileList(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(fl.Entries) != 3 {
		t.Fatalf("預期 3 筆，得到 %d（檔尾沒換行的最後一行可能被吃掉了）", len(fl.Entries))
	}
	if fl.Entries["X1190"].Title != "念佛三昧" || fl.Entries["X1190"].Author != "" {
		t.Errorf("末欄為空的那筆解析錯誤：%+v", fl.Entries["X1190"])
	}
	if fl.Version != "2026r2" {
		t.Errorf("版本解析錯誤：%q", fl.Version)
	}
}

// 經名中的 Unicode 擴充區罕見字（catalog 版用它取代 epub 的組字式）必須原樣保留。
func TestRareUnicodeTitleSurvives(t *testing.T) {
	const title = "大蕃沙洲釋門教法和尚洪𭮨修功德記" // 𭮨 = U+2DBA8
	fl := &FileList{Entries: map[string]Entry{"T2862": {ID: "T2862", Title: title}}}
	p := buildPlan("/root", fl, bookDirs("T/T2862"))
	if got := p.Items[0].NewName; got != "T2862_"+title {
		t.Errorf("罕見字被改動了：%q", got)
	}
}

// docx/odt 的對照表只收子集，檔案數與筆數相同；
// 這裡確認子集不會讓任何一部典籍被判成「查無經號」。
func TestSubsetFileListMatchesAllBooks(t *testing.T) {
	root := t.TempDir()
	makeDocxTree(t, root, ".docx", "T0001", "T0374")
	fl := &FileList{Entries: map[string]Entry{
		"T0001": {ID: "T0001", Title: "長阿含經"},
		"T0374": {ID: "T0374", Title: "大般涅槃經"},
	}}
	files, _ := collectBooks(root, []string{".docx"})
	p := buildPlan(root, fl, files)
	if n := p.Counts()[ActSkipNoEntry]; n != 0 {
		t.Errorf("子集對照表不該讓任何典籍查無經號，卻有 %d 筆", n)
	}
	if n := p.Counts()[ActRename]; n != 2 {
		t.Errorf("預期 2 筆改名，得到 %d", n)
	}
}
