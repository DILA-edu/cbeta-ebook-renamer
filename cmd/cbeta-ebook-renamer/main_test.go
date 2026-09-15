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
