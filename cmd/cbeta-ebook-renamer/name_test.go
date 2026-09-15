package main

import "testing"

func TestParseBase(t *testing.T) {
	cases := []struct {
		base string
		id   string
		kind nameKind
	}{
		{"T0001", "T0001", kindID},
		{"T1159A", "T1159A", kindID}, // 經號結尾帶字母
		{"Ba001", "Ba001", kindID},   // 大藏經補編的樣式
		{"T0001_長阿含經", "T0001", kindRenamed},
		{"T1159A_[峚-大+(企-止)]𡇪大道心驅策法", "T1159A", kindRenamed},
		// docx/odt 的按卷分檔：樣式與「經號_經名」相同，必須判成 kindJuan，
		// 否則改名會把卷號當經名蓋掉、還原會把卷號整個砍掉。
		{"T0001_001", "T0001", kindJuan},
		{"T0099_050", "T0099", kindJuan},
		{"T1159A_012", "T1159A", kindJuan},
		{"cover", "", kindUnknown},
		{"", "", kindUnknown},
	}
	for _, c := range cases {
		id, kind := parseBase(c.base)
		if id != c.id || kind != c.kind {
			t.Errorf("parseBase(%q) = (%q,%v)，預期 (%q,%v)", c.base, id, kind, c.id, c.kind)
		}
	}
}

func TestTargetName(t *testing.T) {
	cases := []struct{ id, title, ext, want string }{
		{"T0001", "長阿含經", ".epub", "T0001_長阿含經.epub"},
		{"T0001", "長阿含經", ".pdf", "T0001_長阿含經.pdf"},
		// 組字式：方括號、圓括號、加減號在兩個 OS 都是合法檔名字元，應原樣保留
		{"T1159A", "[峚-大+(企-止)]𡇪大道心驅策法", ".epub", "T1159A_[峚-大+(企-止)]𡇪大道心驅策法.epub"},
		// 全形空格合法，保留原樣
		{"Y0001", "太虛大師全書．第一編　佛法總學", ".epub", "Y0001_太虛大師全書．第一編　佛法總學.epub"},
		// 檔案系統不允許的字元要換成底線
		{"X0001", "甲/乙:丙*丁", ".epub", "X0001_甲_乙_丙_丁.epub"},
		// Windows 會自動砍掉結尾的半形句點與空白
		{"X0002", "測試經. ", ".epub", "X0002_測試經.epub"},
		// 經名為空時退回只用經號
		{"X0003", "   ", ".epub", "X0003.epub"},
	}
	for _, c := range cases {
		got, _ := targetName(c.id, c.title, c.ext)
		if got != c.want {
			t.Errorf("targetName(%q,%q,%q) = %q，預期 %q", c.id, c.title, c.ext, got, c.want)
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
