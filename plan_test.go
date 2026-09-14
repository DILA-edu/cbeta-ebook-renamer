package main

import (
	"os"
	"path/filepath"
	"sort"
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

func TestBuildPlanKeepsDuplicateTitlesApart(t *testing.T) {
	fl := testFileList()
	files := []string{"T/T0001.epub", "T/T0374.epub", "T/T0375.epub", "T/T0376.epub"}
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
	files := []string{"T/T0001_長阿含經.epub"}
	p := buildPlan("/root", fl, files)
	if p.Items[0].Action != ActSkipDone {
		t.Errorf("已改名的檔案應該略過，得到 %v", p.Items[0].Action)
	}
}

func TestBuildPlanUnknownID(t *testing.T) {
	fl := testFileList()
	p := buildPlan("/root", fl, []string{"T/T9999.epub", "T/readme.epub"})
	if p.Items[0].Action != ActSkipNoEntry {
		t.Errorf("對照表查無的經號應標記為 ActSkipNoEntry，得到 %v", p.Items[0].Action)
	}
	if p.Items[1].Action != ActSkipUnknown {
		t.Errorf("無法辨識的檔名應標記為 ActSkipUnknown，得到 %v", p.Items[1].Action)
	}
}

func TestRestorePlan(t *testing.T) {
	fl := testFileList()
	files := []string{"T/T0001_長阿含經.epub", "T/T0374.epub"}
	p := buildRestorePlan("/root", fl, files)
	if p.Items[0].Action != ActRename || p.Items[0].NewName != "T0001.epub" {
		t.Errorf("還原結果不正確：%+v", p.Items[0])
	}
	if p.Items[1].Action != ActSkipDone {
		t.Errorf("原本就是經號檔名的應略過，得到 %v", p.Items[1].Action)
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

	files, err := collectEpubs(root)
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
	files, _ = collectEpubs(root)
	p2 := buildPlan(root, fl, files)
	p2.checkExisting()
	if n := p2.Counts()[ActRename]; n != 0 {
		t.Errorf("重複執行不應再改名，卻有 %d 筆", n)
	}

	// 還原
	files, _ = collectEpubs(root)
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
	files, _ := collectEpubs(root)
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
