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
	jiraCSV := flag.String("csv", "", "JIRAイシューマッピングCSVファイルのパス（指定しない場合は環境変数から取得）")
	attachmentsFolder := flag.String("folder", "", "添付ファイルのフォルダパス（指定しない場合は環境変数から取得）")
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

	utils.LogInfo("JIRA 添付ファイルアップロードツール")

	// 設定の読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.LogError("設定の読み込みに失敗しました: %v", err)
		os.Exit(1)
	}

	// コマンドラインでパスが指定された場合、設定を上書き
	if *jiraCSV != "" {
		cfg.JiraCSV = *jiraCSV
		utils.LogInfo("CSVファイルを指定: %s", cfg.JiraCSV)
	}

	if *attachmentsFolder != "" {
		cfg.AttachmentsFolder = *attachmentsFolder
		utils.LogInfo("添付ファイルフォルダを指定: %s", cfg.AttachmentsFolder)
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
		utils.LogError("JIRAイシューマッピングCSVファイルが見つかりません: %s", cfg.JiraCSV)
		utils.LogError("先に issue_import ツールを実行して、JIRAイシューを作成してください。")
		os.Exit(1)
	}

	// 添付ファイルフォルダの確認
	if _, err := os.Stat(cfg.AttachmentsFolder); os.IsNotExist(err) {
		utils.LogError("添付ファイルフォルダが見つかりません: %s", cfg.AttachmentsFolder)
		os.Exit(1)
	}

	// 添付ファイルのアップロード実行
	utils.LogInfo("添付ファイルのアップロードを開始します...")
	if err := migrationService.UploadAttachments(); err != nil {
		utils.LogError("添付ファイルアップロードエラー: %v", err)
		os.Exit(1)
	}

	// 処理時間の表示
	elapsed := time.Since(startTime)
	utils.LogInfo("添付ファイルのアップロードが完了しました。処理時間: %s", elapsed)
}

// ヘルプメッセージを表示する関数
func printHelp() {
	fmt.Printf(`
JIRA 添付ファイルアップロードツール

使用方法:
  %s [オプション]

オプション:
  -csv ファイル        JIRAイシューマッピングCSV
  -folder パス         添付ファイルのフォルダパス
  -concurrent 数       並列処理の最大数
  -help                このヘルプを表示する

環境変数:
  JIRA_URL            JIRA URL (必須)
  JIRA_EMAIL          JIRA APIアカウントのメールアドレス (必須)
  JIRA_API_TOKEN      JIRA APIトークン (必須)
  JIRA_CSV            JIRAイシューマッピングCSVファイルパス (デフォルト: jira_import_ready.csv)
  ATTACHMENTS_FOLDER  添付ファイルのフォルダパス (デフォルト: attachments)
  MAX_CONCURRENT      並列処理の最大数 (デフォルト: 10)

説明:
  このツールはPivotal Trackerからエクスポートした添付ファイルを
  対応するJIRAイシューにアップロードします。

  添付ファイルは次のフォルダ構造で格納されている必要があります:
    [ATTACHMENTS_FOLDER]/
      ├── [Pivotal ID 1]/
      │     ├── file1.pdf
      │     └── file2.jpg
      └── [Pivotal ID 2]/
            ├── file3.docx
            └── file4.png

  CSVファイルの"JIRA Issue ID"と"JIRA Issue Key"列を使って
  Pivotal IDとJIRAイシューキーの対応関係を特定します。
`, os.Args[0])
}
