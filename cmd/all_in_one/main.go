package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"pivotaltojira/api"
	"pivotaltojira/config"
	"pivotaltojira/services"
	"pivotaltojira/utils"
)

func main() {
	// コマンドラインフラグの定義
	convertOnly := flag.Bool("convert-only", false, "CSVの変換のみを実行する")
	importOnly := flag.Bool("import-only", false, "イシューのインポートのみを実行する")
	attachmentsOnly := flag.Bool("attachments-only", false, "添付ファイルのアップロードのみを実行する")
	maxConcurrent := flag.Int("concurrent", 0, "並列処理の最大数（0の場合は設定ファイルの値を使用）")
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

	// 設定の読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.LogError("設定の読み込みに失敗しました: %v", err)
		os.Exit(1)
	}

	// 並列処理数の上書き（指定された場合のみ）
	if *maxConcurrent > 0 {
		cfg.MaxConcurrent = *maxConcurrent
	}

	utils.LogInfo("Pivotal → JIRA 移行ツール (v1.0.0)")
	utils.LogInfo("設定読み込み完了 (Max Concurrent: %d)", cfg.MaxConcurrent)

	// 必要なサービスの初期化
	jiraClient := api.NewJiraClient(cfg)
	csvProc := services.NewCSVProcessor(cfg)
	migrationService := services.NewMigrationService(cfg, jiraClient, csvProc)

	// 移行の実行
	err = migrationService.RunMigration(*convertOnly, *importOnly, *attachmentsOnly)
	if err != nil {
		utils.LogError("移行処理に失敗しました: %v", err)
		os.Exit(1)
	}

	// 合計実行時間の表示
	elapsed := time.Since(startTime)
	utils.LogInfo("移行処理が完了しました。合計実行時間: %s", elapsed)
}

// ヘルプメッセージを表示する関数
func printHelp() {
	fmt.Printf(`
Pivotal Tracker → JIRA 移行ツール

使用方法:
  %s [オプション]

オプション:
  -convert-only       CSVの変換のみを実行する
  -import-only        イシューのインポートのみを実行する
  -attachments-only   添付ファイルのアップロードのみを実行する
  -concurrent=N       並列処理の最大数を指定する
  -help               このヘルプを表示する

環境変数:
  JIRA_URL            JIRA URL (必須)
  JIRA_EMAIL          JIRA APIアカウントのメールアドレス (必須)
  JIRA_API_TOKEN      JIRA APIトークン (必須)
  JIRA_PROJECT_KEY    JIRAプロジェクトキー (必須)
  JIRA_STORY_POINT_FIELD  JIRAのストーリーポイントフィールドID (デフォルト: customfield_10016)
  PIVOTAL_CSV         Pivotal Trackerから出力したCSVファイルパス (デフォルト: project_history.csv)
  JIRA_CSV            JIRA用に変換したCSVファイルパス (デフォルト: jira_import_ready.csv)
  ATTACHMENTS_FOLDER  添付ファイルのフォルダパス (デフォルト: attachments)
  MAX_CONCURRENT      並列処理の最大数 (デフォルト: 10)

例:
  # すべての処理を実行
  %s

  # CSVの変換のみを実行
  %s -convert-only

  # イシューのインポートのみを実行
  %s -import-only

  # 添付ファイルのアップロードのみを実行
  %s -attachments-only

  # 並列処理の最大数を20に指定して実行
  %s -concurrent=20
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}
