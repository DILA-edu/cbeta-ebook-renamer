package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Entry 代表 filelist 中的一筆典籍資料。
type Entry struct {
	ID     string // 經號，如 T0001
	Volume string // 冊數
	Juan   string // 卷數
	Title  string // 經名
	Author string // 作譯者
}

// FileList 是整份對照表，以及它的來源資訊。
type FileList struct {
	Path    string           // filelist 檔案的完整路徑
	Version string           // 從檔名解析出的版本，如 2026r2
	Entries map[string]Entry // 以經號為索引
	Order   []string         // 維持 filelist 中的原始順序
}

// filelist_2026r2.txt → 2026r2
var reFileListName = regexp.MustCompile(`^filelist_(.+)\.txt$`)

// findFileList 在指定資料夾中尋找 filelist_*.txt。
// 刻意不寫死版本號，未來 CBETA 出新版也不需要改程式。
func findFileList(root string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "filelist_*.txt"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("在 %s 找不到 filelist_*.txt 對照表", root)
	}
	sort.Strings(matches)
	if len(matches) > 1 {
		warnf("找到多份對照表，採用 %s", filepath.Base(matches[len(matches)-1]))
	}
	// 有多份時取排序最後一份（通常就是最新版）
	return matches[len(matches)-1], nil
}

// loadFileList 解析 CBETA 的對照表。
// 格式：前兩行是標題列與分隔線，其餘每行以逗號分隔為 5 欄，UTF-8 + CRLF。
func loadFileList(path string) (*FileList, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fl := &FileList{
		Path:    path,
		Entries: make(map[string]Entry),
	}
	if m := reFileListName.FindStringSubmatch(filepath.Base(path)); m != nil {
		fl.Version = m[1]
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimRight(sc.Text(), "\r")
		if lineNo == 1 {
			line = strings.TrimPrefix(line, "\ufeff") // 去除可能存在的 BOM
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		// 跳過標題列與 ==== 分隔線
		if strings.HasPrefix(line, "經號") || strings.HasPrefix(line, "==") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			warnf("第 %d 行欄位不足，已略過：%s", lineNo, line)
			continue
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		e := Entry{ID: parts[0], Volume: parts[1], Juan: parts[2], Title: parts[3]}
		if len(parts) >= 5 {
			e.Author = parts[4]
		}
		if e.ID == "" || e.Title == "" {
			warnf("第 %d 行缺少經號或經名，已略過：%s", lineNo, line)
			continue
		}
		if _, dup := fl.Entries[e.ID]; dup {
			warnf("第 %d 行經號 %s 重複，保留先出現的那筆", lineNo, e.ID)
			continue
		}
		fl.Entries[e.ID] = e
		fl.Order = append(fl.Order, e.ID)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(fl.Entries) == 0 {
		return nil, fmt.Errorf("對照表 %s 解析後沒有任何資料", path)
	}
	return fl, nil
}
