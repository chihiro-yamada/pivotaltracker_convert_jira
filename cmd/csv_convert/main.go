package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"pivotaltojira/config"
	"pivotaltojira/services"
	"pivotaltojira/utils"
)

func main() {
	// コマンドラインフラグの定義
	pivotalCSV := flag.String("input", "", "Pivotal Tracker CSVファイルのパス（指定しない場合は環境変数から取得）")
	jiraCSV := flag.String("output", "", "JIRA用に変換されたCSVの出力先（指定しない場合は環境変数から取得）")
	help := flag.Bool("help", false, "ヘルプを表示する")

	// フラグのパース
	flag.Parse()

	// ヘルプフラグが指定された場合はヘルプを表示
	if *help {
		printHelp()
		return
	}

	// 開始時間の記録
	startTime := time.Now()

	utils.LogInfo("Pivotal CSV → JIRA CSV 変換ツール")

	// 設定の読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.LogError("設定の読み込みに失敗しました: %v", err)
		os.Exit(1)
	}

	// コマンドラインでパスが指定された場合、設定を上書き
	if *pivotalCSV != "" {
		cfg.PivotalCSV = *pivotalCSV
		utils.LogInfo("入力ファイルを指定: %s", cfg.PivotalCSV)
	}

	if *jiraCSV != "" {
		cfg.JiraCSV = *jiraCSV
		utils.LogInfo("出力ファイルを指定: %s", cfg.JiraCSV)
	}

	// CSVプロセッサの初期化
	csvProc := services.NewCSVProcessor(cfg)

	// Pivotal CSVの読み込み
	utils.LogInfo("Pivotal CSVを読み込んでいます: %s", cfg.PivotalCSV)
	records, err := csvProc.ReadPivotalCSV()
	if err != nil {
		utils.LogError("Pivotal CSV読み込みエラー: %v", err)
		os.Exit(1)
	}
	utils.LogInfo("Pivotal CSVを読み込みました: %d 件のレコード", len(records))

	// JIRA形式に変換
	utils.LogInfo("JIRAフォーマットに変換しています...")
	jiraRecords, err := csvProc.ProcessPivotalToJiraCSV(records)
	if err != nil {
		utils.LogError("CSV変換エラー: %v", err)
		os.Exit(1)
	}

	// JIRA CSVとして保存
	utils.LogInfo("JIRA CSVとして保存しています: %s", cfg.JiraCSV)
	if err := csvProc.WriteJiraCSV(jiraRecords); err != nil {
		utils.LogError("JIRA CSV書き込みエラー: %v", err)
		os.Exit(1)
	}

	// 処理時間の表示
	elapsed := time.Since(startTime)
	utils.LogInfo("CSV変換が完了しました: %d 件のレコードを処理しました。処理時間: %s", len(jiraRecords), elapsed)
}

// ヘルプメッセージを表示する関数
func printHelp() {
	fmt.Printf(`
Pivotal CSV → JIRA CSV 変換ツール

使用方法:
  %s [オプション]

オプション:
  -input ファイル      入力するPivotal CSV
  -output ファイル     出力するJIRA CSV
  -help               このヘルプを表示する

環境変数:
  PIVOTAL_CSV         Pivotal Trackerから出力したCSVファイルパス (デフォルト: project_history.csv)
  JIRA_CSV            JIRA用に変換したCSVファイルパス (デフォルト: jira_import_ready.csv)

説明:
  このツールはPivotal Trackerからエクスポートしたプロジェクト履歴CSVを
  JIRA用のフォーマットに変換します。

  変換されたCSVファイルは、次のステップであるJIRAイシュー作成の入力として使用されます。
`, os.Args[0])
}
