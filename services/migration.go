package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pivotaltojira/api"
	"pivotaltojira/config"
	"pivotaltojira/models"
	"pivotaltojira/utils"
)

// MigrationService はPivotalからJIRAへのタスク移行を処理します
type MigrationService struct {
	config     *config.Config
	jiraClient *api.JiraClient
	csvProc    *CSVProcessor
}

// NewMigrationService は新しい移行サービスを作成します
func NewMigrationService(cfg *config.Config, jiraClient *api.JiraClient, csvProc *CSVProcessor) *MigrationService {
	return &MigrationService{
		config:     cfg,
		jiraClient: jiraClient,
		csvProc:    csvProc,
	}
}

// ConvertCSV はPivotalのCSVをJIRA形式に変換します
func (m *MigrationService) ConvertCSV() error {
	// Pivotal CSVの読み込み
	records, err := m.csvProc.ReadPivotalCSV()
	if err != nil {
		return fmt.Errorf("Pivotal CSV読み込みエラー: %w", err)
	}

	// JIRA形式に変換
	jiraRecords, err := m.csvProc.ProcessPivotalToJiraCSV(records)
	if err != nil {
		return fmt.Errorf("CSV変換エラー: %w", err)
	}

	// JIRA CSVとして保存
	if err := m.csvProc.WriteJiraCSV(jiraRecords); err != nil {
		return fmt.Errorf("JIRA CSV書き込みエラー: %w", err)
	}

	utils.LogInfo("CSVの変換が完了しました")
	return nil
}

// ImportIssues はJIRAにイシューをインポートします
func (m *MigrationService) ImportIssues() error {
	startTime := time.Now()
	defer utils.TrackTime(startTime, "イシューインポート")

	// JIRA CSVを読み込む
	records, err := m.csvProc.ReadCSV(m.config.JiraCSV)
	if err != nil {
		return fmt.Errorf("JIRA CSV読み込みエラー: %w", err)
	}

	utils.LogInfo("イシューのインポートを開始します: %d 件", len(records))

	// 結果を格納するマップ
	resultMapping := make(models.IssueMapping)
	var resultMutex sync.Mutex

	// エラーフラグを格納するマップ
	errorFlags := make(map[string]bool)
	var errorMutex sync.Mutex

	// セマフォとしてのチャネル（並列数を制限）
	semaphore := make(chan struct{}, m.config.MaxConcurrent)

	// 待機グループ
	var wg sync.WaitGroup

	// エラー数カウンター
	errorCount := 0

	// 各レコードを処理
	for i, record := range records {
		wg.Add(1)

		// セマフォに空構造体を送信（空きスロットを一つ使用）
		semaphore <- struct{}{}

		go func(idx int, rec models.CSVRecord) {
			defer wg.Done()
			defer func() { <-semaphore }() // 処理完了時にセマフォからスロットを解放

			// エラーフラグをチェック（前回の実行で失敗したかどうか）
			if errorFlag, ok := rec["Error"]; ok && errorFlag == "1" {
				utils.LogInfo("行 %d: 前回失敗したレコードを再処理します", idx+1)
			}

			// イシュー作成
			issueKey, err := m.processRecord(rec)

			resultMutex.Lock()
			defer resultMutex.Unlock()

			pivotalID := rec["JIRA Issue ID"]
			if err != nil {
				utils.LogError("行 %d の処理に失敗: %v", idx+1, err)

				errorMutex.Lock()
				errorCount++
				errorFlags[pivotalID] = true
				errorMutex.Unlock()

				resultMapping[pivotalID] = "ERROR"
			} else {
				utils.LogInfo("行 %d の処理が完了: %s", idx+1, issueKey)
				resultMapping[pivotalID] = issueKey

				errorMutex.Lock()
				errorFlags[pivotalID] = false
				errorMutex.Unlock()
			}
		}(i, record)
	}

	// すべてのgoroutineの完了を待つ
	wg.Wait()
	close(semaphore)

	// 結果をCSVに書き込む
	if err := m.csvProc.UpdateJiraKeysWithErrorFlags(resultMapping, errorFlags); err != nil {
		return fmt.Errorf("JIRA キー更新エラー: %w", err)
	}

	utils.LogInfo("イシューのインポートが完了しました: 成功=%d, 失敗=%d", len(resultMapping)-errorCount, errorCount)
	return nil
}

// processRecord は1つのレコードを処理しJIRAイシューを作成します
func (m *MigrationService) processRecord(record models.CSVRecord) (string, error) {
	// 基本情報の取得
	summary := record["Title"]
	if summary == "" {
		summary = "No Title"
	}

	pivotalId := record["JIRA Issue ID"]
	summary = fmt.Sprintf("[%s] %s", pivotalId, summary)
	description := record["Description"]

	// ラベルの処理
	var labels []string
	if labelsStr := record["Labels"]; labelsStr != "" {
		labels = strings.Split(labelsStr, ",")
		for i := range labels {
			labels[i] = strings.TrimSpace(labels[i])
		}
	}

	// 3. 担当者と報告者の処理
    reporter := record["Reporter"]
    assignee := record["Assignee"]

	// イシュータイプの決定
	issueType := "Task" // デフォルト
	if recType, ok := record["Type"]; ok && recType != "" {
		switch strings.ToLower(recType) {
		case "bug":
			issueType = "Bug"
		case "feature", "story":
			issueType = "feature"
		case "chore":
			issueType = "chore"
		case "epic":
			issueType = "Epic"
		case "release":
			issueType = "release"
		}
	}

	// イシュー作成
	issueKey, err := m.jiraClient.CreateIssue(summary, description, labels, issueType, reporter, assignee)
	if err != nil {
		return "", fmt.Errorf("イシュー作成エラー: %w", err)
	}

	// 1. ストーリーポイントの更新
	if spStr := record["Story Points"]; spStr != "" {
		sp := 0
		fmt.Sscanf(spStr, "%d", &sp)
		if sp > 0 {
			if err := m.jiraClient.UpdateStoryPoints(issueKey, sp); err != nil {
				utils.LogWarn("ストーリーポイント設定失敗 %s: %v", issueKey, err)
			}
		}
	}

	// 2. ステータスの更新
	if status := record["JIRA Status"]; status != "" && status != "Backlog" {
		if err := m.jiraClient.UpdateStatus(issueKey, status); err != nil {
			utils.LogWarn("ステータス更新失敗 %s: %v", issueKey, err)
		}
	}

	// 3. コメントの追加
	if comment := record["Comment"]; comment != "" {
		if err := m.jiraClient.AddComment(issueKey, comment); err != nil {
			utils.LogWarn("コメント追加失敗 %s: %v", issueKey, err)
		} else {
			utils.LogInfo("コメントをイシュー %s に追加しました", issueKey)
		}
	}

	return issueKey, nil
}

// UploadAttachments は添付ファイルをアップロードします
func (m *MigrationService) UploadAttachments() error {
	startTime := time.Now()
	defer utils.TrackTime(startTime, "添付ファイルアップロード")

	// イシューマッピングを読み込む
	issueMapping, err := m.csvProc.LoadIssueMapping()
	if err != nil {
		return fmt.Errorf("イシューマッピング読み込みエラー: %w", err)
	}

	// 添付ファイルフォルダの確認
	attachmentsFolder := m.config.AttachmentsFolder
	if _, err := os.Stat(attachmentsFolder); os.IsNotExist(err) {
		return fmt.Errorf("添付ファイルフォルダが見つかりません: %s", attachmentsFolder)
	}

	utils.LogInfo("添付ファイルのアップロードを開始します: フォルダ=%s", attachmentsFolder)

	// セマフォとしてのチャネル（並列数を制限）
	semaphore := make(chan struct{}, m.config.MaxConcurrent)

	// 待機グループ
	var wg sync.WaitGroup

	// カウンター用の変数
	totalFiles := 0
	uploadedFiles := 0
	failedFiles := 0
	var countMutex sync.Mutex

	// サブフォルダ（Pivotal ID）をスキャン
	entries, err := os.ReadDir(attachmentsFolder)
	if err != nil {
		return fmt.Errorf("フォルダ読み取りエラー: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // ファイルはスキップ
		}

		pivotalID := entry.Name()
		issueKey, ok := issueMapping[pivotalID]
		if !ok || issueKey == "ERROR" {
			utils.LogWarn("Pivotal ID %s に対応するJIRAイシューが見つかりません", pivotalID)
			continue
		}

		// サブフォルダ内のファイルをスキャン
		issueFolder := filepath.Join(attachmentsFolder, pivotalID)
		files, err := os.ReadDir(issueFolder)
		if err != nil {
			utils.LogError("フォルダ %s の読み取りエラー: %v", issueFolder, err)
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue // サブフォルダはスキップ
			}

			countMutex.Lock()
			totalFiles++
			countMutex.Unlock()

			filePath := filepath.Join(issueFolder, file.Name())

			wg.Add(1)
			semaphore <- struct{}{} // セマフォ取得

			go func(fPath, iKey string) {
				defer wg.Done()
				defer func() { <-semaphore }() // セマフォ解放

				// 添付ファイルのアップロード
				err := m.jiraClient.UploadAttachment(iKey, fPath)

				countMutex.Lock()
				defer countMutex.Unlock()

				if err != nil {
					utils.LogError("ファイル %s のアップロード失敗: %v", fPath, err)
					failedFiles++
				} else {
					utils.LogInfo("ファイル %s をイシュー %s にアップロードしました", filepath.Base(fPath), iKey)
					uploadedFiles++
				}
			}(filePath, issueKey)
		}
	}

	// すべてのgoroutineの完了を待つ
	wg.Wait()
	close(semaphore)

	utils.LogInfo("添付ファイルのアップロードが完了しました: 合計=%d, 成功=%d, 失敗=%d",
		totalFiles, uploadedFiles, failedFiles)

	return nil
}

// RunMigration は移行処理全体を実行します
func (m *MigrationService) RunMigration(convertOnly, importOnly, attachmentsOnly bool) error {
	startTime := time.Now()
	defer utils.TrackTime(startTime, "移行処理全体")

	// JIRA認証チェック
	if err := m.jiraClient.CheckAuth(); err != nil {
		return fmt.Errorf("JIRA認証エラー: %w", err)
	}

	utils.LogInfo("JIRA認証成功")

	// 全処理またはCSV変換のみ
	if !importOnly && !attachmentsOnly {
		utils.LogInfo("CSVデータの変換を開始します")
		if err := m.ConvertCSV(); err != nil {
			return err
		}
	}

	// 変換のみの場合はここで終了
	if convertOnly {
		return nil
	}

	// 全処理またはイシューインポートのみ
	if !attachmentsOnly {
		utils.LogInfo("JIRAイシューのインポートを開始します")
		if err := m.ImportIssues(); err != nil {
			return err
		}
	}

	// 全処理または添付ファイルアップロードのみ
	if !importOnly || attachmentsOnly {
		utils.LogInfo("添付ファイルのアップロードを開始します")
		if err := m.UploadAttachments(); err != nil {
			return err
		}
	}

	utils.LogInfo("移行処理が完了しました")
	return nil
}
