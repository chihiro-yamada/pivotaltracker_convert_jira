package main

import (
	"flag"
	"fmt"
	"os"

	"pivotaltojira/api"
	"pivotaltojira/config"
	"pivotaltojira/utils"
)

func main() {
	// ヘルプフラグの定義
	help := flag.Bool("help", false, "ヘルプを表示する")

	// フラグのパース
	flag.Parse()

	// ヘルプフラグが指定された場合はヘルプを表示
	if *help {
		printHelp()
		return
	}

	utils.LogInfo("JIRA認証確認ツール")

	// 設定の読み込み
	cfg, err := config.LoadConfig()
	if err != nil {
		utils.LogError("設定の読み込みに失敗しました: %v", err)
		os.Exit(1)
	}

	// JIRAクライアントの初期化
	jiraClient := api.NewJiraClient(cfg)

	// 認証チェック
	utils.LogInfo("JIRA APIの認証を確認しています...")
	err = jiraClient.CheckAuth()
	if err != nil {
		utils.LogError("JIRA認証エラー: %v", err)
		utils.LogError("認証情報を確認してください。")
		os.Exit(1)
	}

	utils.LogInfo("JIRA認証成功！ 接続先: %s", cfg.JiraURL)
	utils.LogInfo("JIRA APIの認証情報は正常です。")
}

// ヘルプメッセージを表示する関数
func printHelp() {
	fmt.Printf(`
JIRA認証確認ツール

使用方法:
  %s [オプション]

オプション:
  -help               このヘルプを表示する

環境変数:
  JIRA_URL            JIRA URL (必須)
  JIRA_EMAIL          JIRA APIアカウントのメールアドレス (必須)
  JIRA_API_TOKEN      JIRA APIトークン (必須)

説明:
  このツールはJIRA APIの認証情報が正しく設定されているかを確認します。
  認証が成功すれば、他のツールも正常に動作する可能性が高いです。
`, os.Args[0])
}
