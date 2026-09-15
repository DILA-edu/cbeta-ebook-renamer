package main

import "testing"

// 資料夾名稱的版本字串解析：docx/odt 新版會改成 cbeta_docx_2026r2/，
// 與 epub、pdf 共用同一條規則。
func TestDirVersionParsing(t *testing.T) {
	for _, tc := range []struct{ dir, want string }{
		{"cbeta_epub_2026r2", "2026r2"},
		{"cbeta_pdf_1_2026r2", "2026r2"}, // _1 是 PDF 分三次下載的批次編號
		{"cbeta_docx_2026r2", "2026r2"},
		{"cbeta_odt_2026r2", "2026r2"},
		{"docx", ""}, // 舊結構沒有版本字串，不比對
		{"odt", ""},
	} {
		m := reDirVersion.FindStringSubmatch(tc.dir)
		got := ""
		if m != nil {
			got = m[1]
		}
		if got != tc.want {
			t.Errorf("%s 解析出版本 %q，預期 %q", tc.dir, got, tc.want)
		}
	}
}

// 雙擊執行時「該不該改問還原」的判準。
// 只有「整批都已改過名」才問；混合狀態照舊直接走正向改名。
func TestLooksRenamed(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    map[Action]int
		want bool
	}{
		{"整批已改過名", map[Action]int{ActSkipDone: 2920}, true},
		{"全新未改", map[Action]int{ActRename: 2920}, false},
		{"混合狀態", map[Action]int{ActRename: 10, ActSkipDone: 2910}, false},
		{"已改過名但有衝突", map[Action]int{ActSkipDone: 2919, ActConflict: 1}, true},
		{"查無經號的雜檔", map[Action]int{ActSkipNoEntry: 3}, false},
		{"空資料夾", map[Action]int{}, false},
	} {
		if got := looksRenamed(tc.c); got != tc.want {
			t.Errorf("%s：looksRenamed = %v，預期 %v", tc.name, got, tc.want)
		}
	}
}
