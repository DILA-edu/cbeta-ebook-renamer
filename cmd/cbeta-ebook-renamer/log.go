package main

import (
	"encoding/csv"
	"os"
	"time"
)

// logWriter 產生改名紀錄 CSV，方便使用者事後查核或自行還原。
type logWriter struct {
	f  *os.File
	w  *csv.Writer
	op string
}

func newLogWriter(path, op string) (*logWriter, error) {
	if path == "" {
		return &logWriter{}, nil
	}
	// 以附加模式開啟：改名與後續的還原紀錄都要留著，才能追溯完整經過。
	_, statErr := os.Stat(path)
	isNew := statErr != nil

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	if isNew {
		// 加上 BOM，Windows 的 Excel 才不會把 UTF-8 中文讀成亂碼。
		if _, err := f.WriteString("\ufeff"); err != nil {
			f.Close()
			return nil, err
		}
	}
	w := csv.NewWriter(f)
	if isNew {
		if err := w.Write([]string{"時間", "操作", "資料夾", "原檔名", "新檔名", "結果"}); err != nil {
			f.Close()
			return nil, err
		}
		w.Flush()
	}
	return &logWriter{f: f, w: w, op: op}, nil
}

func (l *logWriter) write(it Item, result string) {
	if l.w == nil {
		return
	}
	dir := it.Dir
	if dir == "" {
		dir = "."
	}
	// 資料夾項目的名稱後面帶斜線，紀錄檔中才分得出改的是資料夾還是檔案
	_ = l.w.Write([]string{
		time.Now().Format("2006-01-02 15:04:05"),
		l.op, dir, it.DisplayOld(), it.DisplayNew(), result,
	})
}

func (l *logWriter) close() {
	if l.w != nil {
		l.w.Flush()
	}
	if l.f != nil {
		l.f.Close()
		l.f = nil
		l.w = nil
	}
}
