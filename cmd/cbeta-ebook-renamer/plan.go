package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Action 是單一檔案的處理結果分類。
type Action int

const (
	ActRename      Action = iota // 需要改名
	ActSkipDone                  // 已經是目標檔名，不需處理
	ActSkipNoEntry               // 對照表中查無此經號
	ActSkipUnknown               // 檔名不符合經號樣式，無法判斷
	ActSkipJuan                  // 按卷分檔（docx/odt），本版本不處理
	ActConflict                  // 目標檔名衝突，不予處理
)

func (a Action) String() string {
	switch a {
	case ActRename:
		return "改名"
	case ActSkipDone:
		return "略過(已改名)"
	case ActSkipNoEntry:
		return "略過(查無經號)"
	case ActSkipUnknown:
		return "略過(檔名無法辨識)"
	case ActSkipJuan:
		return "略過(按卷分檔)"
	case ActConflict:
		return "衝突"
	}
	return "未知"
}

// Item 是一個典籍檔案的處理計畫。
type Item struct {
	Dir     string // 相對根目錄的子資料夾，如 T
	OldName string
	NewName string
	ID      string
	Action  Action
	Note    string
}

// Plan 是整批作業的計畫。
type Plan struct {
	Root  string
	Items []Item
}

// Counts 回傳各類動作的數量統計。
func (p *Plan) Counts() map[Action]int {
	c := make(map[Action]int)
	for _, it := range p.Items {
		c[it.Action]++
	}
	return c
}

// collectBooks 掃出根目錄下所有符合指定副檔名的檔案。
// exts 需為小寫、含點，例如 []string{".epub", ".pdf"}。
func collectBooks(root string, exts []string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			warnf("讀取 %s 失敗：%v", path, err)
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			// 略過隱藏資料夾（.git、.DS_Store 之類的環境產物）
			if path != root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if matchExt(name, exts) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			found = append(found, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(found)
	return found, nil
}

func matchExt(name string, exts []string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

// buildPlan 規劃「經號 → 經號_經名」的改名作業。
func buildPlan(root string, fl *FileList, files []string) *Plan {
	p := &Plan{Root: root}
	// 以「資料夾 + 小寫目標檔名」為鍵偵測衝突。
	// 用小寫是因為 APFS 與 NTFS 預設不分大小寫，只比對原字串會漏抓。
	taken := make(map[string]string)

	for _, rel := range files {
		dir := filepath.Dir(rel)
		if dir == "." {
			dir = ""
		}
		old := filepath.Base(rel)
		ext := filepath.Ext(old)
		base := strings.TrimSuffix(old, ext)

		id, kind := parseBase(base)
		switch kind {
		case kindUnknown:
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, Action: ActSkipUnknown})
			continue
		case kindJuan:
			// docx/odt 的按卷分檔，形狀與「經號_經名」相同但語意不同，
			// 改名會把卷號當成經名蓋掉，必須明確跳過。
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, ID: id, Action: ActSkipJuan,
				Note: "按卷分檔的檔案，本版本尚未支援"})
			continue
		}
		entry, exists := fl.Entries[id]
		if !exists {
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, ID: id, Action: ActSkipNoEntry,
				Note: "對照表中沒有這個經號"})
			continue
		}
		newName, notes := targetName(id, entry.Title, ext)
		note := strings.Join(notes, "；")

		if old == newName {
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, NewName: newName, ID: id,
				Action: ActSkipDone})
			continue
		}
		if kind == kindRenamed {
			// 已改過名但與目前對照表不一致，可能是換了新版 filelist，仍予以更新。
			note = joinNote(note, "既有檔名與對照表不符，將更新")
		}

		key := dir + "\x00" + strings.ToLower(newName)
		if prev, dup := taken[key]; dup {
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, NewName: newName, ID: id,
				Action: ActConflict, Note: "目標檔名與 " + prev + " 相同"})
			continue
		}
		taken[key] = old
		p.Items = append(p.Items, Item{Dir: dir, OldName: old, NewName: newName, ID: id,
			Action: ActRename, Note: note})
	}
	return p
}

// buildRestorePlan 規劃還原作業：「經號_經名 → 經號」。
func buildRestorePlan(root string, fl *FileList, files []string) *Plan {
	p := &Plan{Root: root}
	taken := make(map[string]string)

	for _, rel := range files {
		dir := filepath.Dir(rel)
		if dir == "." {
			dir = ""
		}
		old := filepath.Base(rel)
		ext := filepath.Ext(old)
		base := strings.TrimSuffix(old, ext)

		id, kind := parseBase(base)
		switch kind {
		case kindUnknown:
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, Action: ActSkipUnknown})
			continue
		case kindJuan:
			// 若誤把卷號當成經名還原，T0001_001.docx 會變成 T0001.docx，卷號就沒了
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, ID: id, Action: ActSkipJuan,
				Note: "按卷分檔的檔案，本版本尚未支援"})
			continue
		}
		if kind == kindID {
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, NewName: old, ID: id,
				Action: ActSkipDone})
			continue
		}
		// 與改名模式一樣要求經號存在於對照表，避免誤動到資料夾裡的其他檔案
		if _, exists := fl.Entries[id]; !exists {
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, ID: id, Action: ActSkipNoEntry,
				Note: "對照表中沒有這個經號"})
			continue
		}
		newName := id + ext
		key := dir + "\x00" + strings.ToLower(newName)
		if prev, dup := taken[key]; dup {
			p.Items = append(p.Items, Item{Dir: dir, OldName: old, NewName: newName, ID: id,
				Action: ActConflict, Note: "目標檔名與 " + prev + " 相同"})
			continue
		}
		taken[key] = old
		p.Items = append(p.Items, Item{Dir: dir, OldName: old, NewName: newName, ID: id,
			Action: ActRename})
	}
	return p
}

// checkExisting 檢查目標檔名是否已經被「不在這次計畫中」的檔案佔用。
func (p *Plan) checkExisting() {
	moving := make(map[string]bool)
	for _, it := range p.Items {
		if it.Action == ActRename {
			moving[filepath.Join(it.Dir, strings.ToLower(it.OldName))] = true
		}
	}
	for i := range p.Items {
		it := &p.Items[i]
		if it.Action != ActRename {
			continue
		}
		target := filepath.Join(p.Root, it.Dir, it.NewName)
		if _, err := os.Stat(target); err == nil {
			// 目標存在；若它本身也正要被改走就沒問題
			if !moving[filepath.Join(it.Dir, strings.ToLower(it.NewName))] {
				it.Action = ActConflict
				it.Note = joinNote(it.Note, "目標檔案已存在")
			}
		}
	}
}

// warnLongPaths 對接近 Windows MAX_PATH(260) 的路徑提出警告。
func (p *Plan) warnLongPaths() int {
	n := 0
	for _, it := range p.Items {
		if it.Action != ActRename {
			continue
		}
		full := filepath.Join(p.Root, it.Dir, it.NewName)
		if len([]rune(full)) > 240 {
			n++
			if n <= 5 {
				warnf("路徑偏長（%d 字元），Windows 舊版可能無法開啟：%s", len([]rune(full)), full)
			}
		}
	}
	if n > 5 {
		warnf("另有 %d 個過長路徑未逐一列出", n-5)
	}
	return n
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "；" + b
}

// execute 實際執行改名，回傳成功數與錯誤清單。
func (p *Plan) execute(logw *logWriter) (int, []error) {
	var errs []error
	done := 0
	for _, it := range p.Items {
		// 衝突的項目雖然不處理，仍寫進紀錄，否則使用者關掉視窗就查不到了
		if it.Action == ActConflict {
			logw.write(it, "未處理（衝突）："+it.Note)
			continue
		}
		if it.Action != ActRename {
			continue
		}
		oldPath := filepath.Join(p.Root, it.Dir, it.OldName)
		newPath := filepath.Join(p.Root, it.Dir, it.NewName)
		if err := os.Rename(oldPath, newPath); err != nil {
			errs = append(errs, fmt.Errorf("%s：%w", oldPath, err))
			logw.write(it, "失敗："+err.Error())
			continue
		}
		done++
		logw.write(it, "成功")
	}
	return done, errs
}
