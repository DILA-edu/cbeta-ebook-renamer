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

// targetName 依「經號_經名.epub」規則產生新檔名。
func targetName(id, title string) (string, []string) {
	clean, notes := sanitizeTitle(title)
	if clean == "" {
		notes = append(notes, "經名處理後為空字串，維持只用經號")
		return id + ".epub", notes
	}
	return id + "_" + clean + ".epub", notes
}

// parseBase 從主檔名判斷它的經號，以及是否已經改過名。
func parseBase(base string) (id string, renamed bool, ok bool) {
	if reBookID.MatchString(base) {
		return base, false, true
	}
	if m := reRenamed.FindStringSubmatch(base); m != nil {
		return m[1], true, true
	}
	return "", false, false
}
