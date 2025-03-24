# PivotalTracker to JIRA Migration Tool

このツールは、PivotalTrackerからJIRAへプロジェクトを移行するためのGoアプリケーションです。タスク、ストーリーポイント、ステータス、添付ファイルなどをPivotalTrackerからエクスポートして、JIRAに効率的にインポートします。

## 特徴

- PivotalTrackerのCSVデータをJIRA形式に変換
- JIRAへのイシュー（タスク）の作成
- ストーリーポイントとステータスの設定
- 添付ファイルのアップロード
- 並列処理による高速な移行処理

## 必要条件

- JIRAアカウントとAPIトークン
- PivotalTrackerからエクスポートしたCSVファイル

## フォルダ構成

```
pivotaltojira/
├── cmd/                    # コマンドラインツール
│   ├── all_in_one/         # 一括実行ツール
│   ├── auth_check/         # 認証確認ツール
│   ├── csv_convert/        # CSV変換ツール
│   ├── issue_import/       # イシューインポートツール
│   └── attachment_upload/  # 添付ファイルアップロードツール
├── config/                 # 設定管理
│   └── config.go
├── models/                 # データモデル
│   └── models.go
├── api/                    # API通信
│   └── jira_client.go
├── services/               # ビジネスロジック
│   ├── csv_processor.go    # CSV処理
│   └── migration.go        # 移行処理
├── utils/                  # ユーティリティ
│   └── logger.go           # ログ機能
├── .env                    # 環境変数設定（作成が必要）
├── .env.example            # 環境変数のサンプル
├── go.mod                  # Go モジュール定義
├── go.sum                  # 依存関係のハッシュ
└── README.md               # このファイル
```

## インストール

```bash
# リポジトリをクローン
git clone https://github.com/yourusername/pivotaltojira.git
cd pivotaltojira

# 依存パッケージのインストール
go mod download
