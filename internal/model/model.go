package model

import "time"

// LogLine 表示采集到的一行日志。
type LogLine struct {
	FilePath string
	Line     int64
	Time     time.Time
	Content  string
}

