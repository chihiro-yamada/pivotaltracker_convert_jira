package utils

import (
	"log"
	"os"
	"time"
)

var (
	// InfoLogger は情報レベルのログを出力します
	InfoLogger *log.Logger
	// WarnLogger は警告レベルのログを出力します
	WarnLogger *log.Logger
	// ErrorLogger はエラーレベルのログを出力します
	ErrorLogger *log.Logger
)

// init関数はパッケージがインポートされたときに自動的に実行されます
func init() {
	InfoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	WarnLogger = log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime)
	ErrorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime)
}

// LogInfo は情報レベルのメッセージをログに記録します
func LogInfo(format string, v ...interface{}) {
	InfoLogger.Printf(format, v...)
}

// LogWarn は警告レベルのメッセージをログに記録します
func LogWarn(format string, v ...interface{}) {
	WarnLogger.Printf(format, v...)
}

// LogError はエラーレベルのメッセージをログに記録します
func LogError(format string, v ...interface{}) {
	ErrorLogger.Printf(format, v...)
}

// TrackTime は関数の実行時間を計測して出力するユーティリティです
func TrackTime(start time.Time, name string) {
	elapsed := time.Since(start)
	LogInfo("%s 完了時間: %s", name, elapsed)
}
