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
	jiraCSV := flag.String("input", "", "JIRAインポート用CSVファイルのパス（指定しない場合は環境変数から取得）")
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

	utils.LogInfo("JIRA イシューインポートツール")

	// 設定の読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.LogError("設定の読み込みに失敗しました: %v", err)
		os.Exit(1)
	}

	// コマンドラインでパスが指定された場合、設定を上書き
	if *jiraCSV != "" {
		cfg.JiraCSV = *jiraCSV
		utils.LogInfo("入力ファイルを指定: %s", cfg.JiraCSV)
	}

	// 並列処理数の上書き（指定された場合のみ）
	if *maxConcurrent > 0 {
		cfg.MaxConcurrent = *maxConcurrent
		utils.LogInfo("並列処理数を指定: %d", cfg.MaxConcurrent)
	}

	// JIRA認証情報の確認
	utils.LogInfo("JIRA認証情報を確認しています...")
	jiraClient := api.NewJiraClient(cfg)
	if err := jiraClient.CheckAuth(); err != nil {
		utils.LogError("JIRA認証エラー: %v", err)
		utils.LogError("JIRAの認証情報を確認してください。")
		os.Exit(1)
	}
	utils.LogInfo("JIRA認証成功")

	// CSVプロセッサの初期化
	csvProc := services.NewCSVProcessor(cfg)

	// 移行サービスの初期化
	migrationService := services.NewMigrationService(cfg, jiraClient, csvProc)

	// CSVファイルの存在確認
	if _, err := os.Stat(cfg.JiraCSV); os.IsNotExist(err) {
		utils.LogError("JIRAインポート用CSVファイルが見つかりません: %s", cfg.JiraCSV)
		utils.LogError("先に csv_convert ツールを実行して、CSVを準備してください。")
		os.Exit(1)
	}

	// イシューのインポート実行
	utils.LogInfo("JIRAイシューのインポートを開始します...")
	if err := migrationService.ImportIssues(); err != nil {
		utils.LogError("イシューインポートエラー: %v", err)
		os.Exit(1)
	}

	// 処理時間の表示
	elapsed := time.Since(startTime)
	utils.LogInfo("JIRAイシューのインポートが完了しました。処理時間: %s", elapsed)
}

// ヘルプメッセージを表示する関数
func printHelp() {
	fmt.Printf(`
JIRA イシューインポートツール

使用方法:
  %s [オプション]

オプション:
  -input ファイル      インポートするJIRA CSV
  -concurrent 数      並列処理の最大数
  -help               このヘルプを表示する

環境変数:
  JIRA_URL            JIRA URL (必須)
  JIRA_EMAIL          JIRA APIアカウントのメールアドレス (必須)
  JIRA_API_TOKEN      JIRA APIトークン (必須)
  JIRA_PROJECT_KEY    JIRAプロジェクトキー (必須)
  JIRA_STORY_POINT_FIELD  JIRAのストーリーポイントフィールドID (デフォルト: customfield_10016)
  JIRA_CSV            JIRA用に変換したCSVファイルパス (デフォルト: jira_import_ready.csv)
  MAX_CONCURRENT      並列処理の最大数 (デフォルト: 10)

説明:
  このツールは変換されたCSVファイルからJIRAイシューを作成します。

  作成されたイシューのキー(例: PROJECT-123)はCSVファイルの
  "JIRA Issue Key"列に追加されます。

  並列処理の最大数を増やすとインポート速度が向上しますが、
  JIRAのAPIレート制限に注意してください。
`, os.Args[0])
}
