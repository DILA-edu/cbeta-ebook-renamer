package main

import (
	"regexp"
	"strings"
	"unicode"
)

// 經號的樣式：字母開頭 + 數字，結尾可能再帶一個字母（如 T1159A）。
var reBookID = regexp.MustCompile(`^[A-Za-z]{1,3}[0-9]{1,5}[A-Za-z]?$`)

// 已改名的檔案樣式：經號_經名
var reRenamed = regexp.MustCompile(`^([A-Za-z]{1,3}[0-9]{1,5}[A-Za-z]?)_(.+)$`)

// 按卷分檔的樣式：經號_卷號，如 T0001_001。
// docx 與 odt 版本採用這種命名，形狀與「經號_經名」相同，
// 必須先攔下來，否則會被誤認成已改名的檔案而把卷號當成經名。
var reJuan = regexp.MustCompile(`^([A-Za-z]{1,3}[0-9]{1,5}[A-Za-z]?)_([0-9]{2,4})$`)

// Windows 不允許出現在檔名中的字元。macOS 只禁止 ':' 與 '/'，取兩者聯集最保險。
const illegalChars = `\/:*?"<>|`

// Windows 保留裝置名稱，這些名字（含任何副檔名）都無法建立檔案。
var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// maxBaseBytes 是檔名主檔名的位元組上限。
// 目前最長經名 57 字（約 171 bytes）遠低於此值，這裡純粹是未來版本的保險。
const maxBaseBytes = 200

// sanitizeTitle 把經名轉成在 macOS 與 Windows 都合法的檔名片段。
// 刻意不做 Unicode 正規化（NFC/NFD），原樣保留 CBETA 提供的字串。
func sanitizeTitle(title string) (string, []string) {
	var notes []string
	var b strings.Builder

	replaced := false
	for _, r := range title {
		switch {
		case strings.ContainsRune(illegalChars, r):
			b.WriteRune('_')
			replaced = true
		case r < 0x20 || r == 0x7f:
			b.WriteRune('_')
			replaced = true
		default:
			b.WriteRune(r)
		}
	}
	s := b.String()
	if replaced {
		notes = append(notes, "含檔案系統不允許的字元，已改為底線")
	}

	// Windows 會自動砍掉結尾的半形空白與句點，先自行處理避免預期外的結果。
	trimmed := strings.TrimRight(s, " .")
	trimmed = strings.TrimLeftFunc(trimmed, unicode.IsSpace)
	if trimmed != s {
		notes = append(notes, "已去除頭尾的空白或句點")
	}
	s = trimmed

	// 位元組長度保險（以字元為單位截斷，不切壞 UTF-8）
	if len(s) > maxBaseBytes {
		cut := 0
		for i := range s {
			if i > maxBaseBytes {
				break
			}
			cut = i
		}
		s = s[:cut]
		notes = append(notes, "經名過長，已截斷")
	}

	if reservedNames[strings.ToUpper(s)] {
		s = s + "_"
		notes = append(notes, "與 Windows 保留裝置名稱相同，已加上底線")
	}
	return s, notes
}

// targetName 依「經號_經名」規則產生新檔名，ext 是含點的副檔名（如 .epub）。
func targetName(id, title, ext string) (string, []string) {
	clean, notes := sanitizeTitle(title)
	if clean == "" {
		notes = append(notes, "經名處理後為空字串，維持只用經號")
		return id + ext, notes
	}
	return id + "_" + clean + ext, notes
}

// nameKind 是主檔名的類型。
type nameKind int

const (
	kindUnknown nameKind = iota // 無法辨識
	kindID                      // 純經號，如 T0001
	kindRenamed                 // 已改名，如 T0001_長阿含經
	kindJuan                    // 按卷分檔，如 T0001_001
)

// parseBase 從主檔名判斷它的經號與類型。
func parseBase(base string) (id string, kind nameKind) {
	if reBookID.MatchString(base) {
		return base, kindID
	}
	// 卷號的判斷必須排在「已改名」前面，兩者的樣式會互相涵蓋
	if m := reJuan.FindStringSubmatch(base); m != nil {
		return m[1], kindJuan
	}
	if m := reRenamed.FindStringSubmatch(base); m != nil {
		return m[1], kindRenamed
	}
	return "", kindUnknown
}
