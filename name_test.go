package main

import "testing"

func TestParseBase(t *testing.T) {
	cases := []struct {
		base    string
		id      string
		renamed bool
		ok      bool
	}{
		{"T0001", "T0001", false, true},
		{"T1159A", "T1159A", false, true}, // 經號結尾帶字母
		{"Ba001", "Ba001", false, true},   // 大藏經補編的樣式
		{"T0001_長阿含經", "T0001", true, true},
		{"T1159A_[峚-大+(企-止)]𡇪大道心驅策法", "T1159A", true, true},
		{"cover", "", false, false},
		{"", "", false, false},
	}
	for _, c := range cases {
		id, renamed, ok := parseBase(c.base)
		if id != c.id || renamed != c.renamed || ok != c.ok {
			t.Errorf("parseBase(%q) = (%q,%v,%v)，預期 (%q,%v,%v)",
				c.base, id, renamed, ok, c.id, c.renamed, c.ok)
		}
	}
}

func TestTargetName(t *testing.T) {
	cases := []struct{ id, title, want string }{
		{"T0001", "長阿含經", "T0001_長阿含經.epub"},
		// 組字式：方括號、圓括號、加減號在兩個 OS 都是合法檔名字元，應原樣保留
		{"T1159A", "[峚-大+(企-止)]𡇪大道心驅策法", "T1159A_[峚-大+(企-止)]𡇪大道心驅策法.epub"},
		// 全形空格合法，保留原樣
		{"Y0001", "太虛大師全書．第一編　佛法總學", "Y0001_太虛大師全書．第一編　佛法總學.epub"},
		// 檔案系統不允許的字元要換成底線
		{"X0001", "甲/乙:丙*丁", "X0001_甲_乙_丙_丁.epub"},
		// Windows 會自動砍掉結尾的半形句點與空白
		{"X0002", "測試經. ", "X0002_測試經.epub"},
		// 經名為空時退回只用經號
		{"X0003", "   ", "X0003.epub"},
	}
	for _, c := range cases {
		got, _ := targetName(c.id, c.title)
		if got != c.want {
			t.Errorf("targetName(%q,%q) = %q，預期 %q", c.id, c.title, got, c.want)
		}
	}
}

func TestSanitizeTitleTruncate(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "經" // 每字 3 bytes，共 600 bytes
	}
	got, notes := sanitizeTitle(long)
	if len(got) > maxBaseBytes {
		t.Errorf("截斷後仍有 %d bytes，超過上限 %d", len(got), maxBaseBytes)
	}
	if len(notes) == 0 {
		t.Error("截斷時應該要有提示訊息")
	}
	// 確認沒有切壞 UTF-8
	for _, r := range got {
		if r == '�' {
			t.Error("截斷位置切壞了 UTF-8 字元")
		}
	}
}

func TestReservedName(t *testing.T) {
	got, _ := sanitizeTitle("CON")
	if got != "CON_" {
		t.Errorf("Windows 保留名稱應加底線，得到 %q", got)
	}
}
