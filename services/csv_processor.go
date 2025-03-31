package services

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"pivotaltojira/config"
	"pivotaltojira/models"
	"pivotaltojira/utils"
)

// CSVProcessor はCSVファイルの読み書きを担当します
type CSVProcessor struct {
	config *config.Config
}

// NewCSVProcessor は新しいCSVプロセッサーを作成します
func NewCSVProcessor(cfg *config.Config) *CSVProcessor {
	return &CSVProcessor{
		config: cfg,
	}
}

// ReadPivotalCSV はPivotal CSVを読み込みます
func (p *CSVProcessor) ReadPivotalCSV() ([]models.CSVRecord, error) {
	utils.LogInfo("Pivotal CSVファイル '%s' を読み込みます", p.config.PivotalCSV)

	file, err := os.Open(p.config.PivotalCSV)
	if err != nil {
		return nil, fmt.Errorf("CSVオープンエラー: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // フィールド数の不一致を許可
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV読み込みエラー: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSVデータが不足しています")
	}

	headers := records[0]
	result := make([]models.CSVRecord, 0, len(records)-1)

	// ヘッダーの重複をチェックし、インデックスを記録
	headerIndices := make(map[string][]int)
	for i, header := range headers {
		headerIndices[header] = append(headerIndices[header], i)
	}

	for i, record := range records[1:] {
		// フィールド数のチェック
		if len(record) > len(headers) {
			return nil, fmt.Errorf("行 %d: フィールド数がヘッダー数より多いです（ヘッダー: %d, 行: %d）", i+2, len(headers), len(record))
		} else if len(record) < len(headers) {
			utils.LogWarn("行 %d: フィールド数が不一致（ヘッダー: %d, 行: %d）- 不足分は空にします", i+2, len(headers), len(record))
			// 不足分を埋める
			newRecord := make([]string, len(headers))
			copy(newRecord, record)
			record = newRecord
		}

		rowData := make(models.CSVRecord)

		// 通常のフィールド処理 (Comment以外)
		for header, indices := range headerIndices {
			if header != "Comment" {
				// 他のカラムは最初のインデックスのみ使用
				if len(indices) > 0 && indices[0] < len(record) {
					rowData[header] = record[indices[0]]
				} else {
					rowData[header] = ""
				}
			}
		}

		// Commentフィールドの特別処理（結合）
		if commentIndices, ok := headerIndices["Comment"]; ok && len(commentIndices) > 0 {
			var comments []string
			for _, idx := range commentIndices {
				if idx < len(record) && record[idx] != "" {
					comments = append(comments, record[idx])
				}
			}

			// コメントを区切り線で結合
			if len(comments) > 0 {
				rowData["Comment"] = strings.Join(comments, "\n\n===========================\n\n")
			} else {
				rowData["Comment"] = ""
			}
		}

		result = append(result, rowData)
	}

	utils.LogInfo("Pivotal CSVを読み込みました: %d 行", len(result))
	return result, nil
}

// ProcessPivotalToJiraCSV はPivotalデータをJIRA用に変換します
func (p *CSVProcessor) ProcessPivotalToJiraCSV(records []models.CSVRecord) ([]models.CSVRecord, error) {
	utils.LogInfo("PivotalデータをJIRA形式に変換しています...")

	if len(records) == 0 {
		return nil, fmt.Errorf("処理するデータがありません")
	}

	result := make([]models.CSVRecord, 0, len(records))

	// PivotalからJIRAへの変換処理
	for i, record := range records {
		jiraRecord := make(models.CSVRecord)

		// 基本フィールドをマッピング
		jiraRecord["JIRA Issue ID"] = record["Id"]
		jiraRecord["Title"] = record["Title"]
		jiraRecord["Description"] = record["Description"]
		jiraRecord["Labels"] = record["Labels"]
		jiraRecord["Type"] = record["Type"]

		// ステータスマッピング
		pivotalStatus := strings.ToLower(record["Current State"])
		jiraRecord["JIRA Status"] = config.StatusMapping[pivotalStatus]

		// ストーリーポイント変換
		storyPoints := 0
		if estimate, ok := record["Estimate"]; ok && estimate != "" {
			storyPoints, _ = strconv.Atoi(estimate)
		}
		jiraRecord["Story Points"] = strconv.Itoa(storyPoints)

		// 日付フォーマット変換
		jiraRecord["Created Date"] = p.convertDateFormat(record["Created at"])
		jiraRecord["Resolved Date"] = p.convertDateFormat(record["Accepted at"])

		// 担当者
		jiraRecord["Assignee"] = record["Owned By"]

		// 報告者
		jiraRecord["Reporter"] = record["Requested By"]

		// コメント
		jiraRecord["Comment"] = record["Comment"]


		// JIRA Issue Keyは後で更新
		jiraRecord["JIRA Issue Key"] = ""

		result = append(result, jiraRecord)

		// 進捗を表示（大量データの場合）
		if i > 0 && i%100 == 0 {
			utils.LogInfo("処理中... %d/%d 行完了", i, len(records))
		}
	}

	utils.LogInfo("変換完了: %d 行を処理しました", len(result))
	return result, nil
}

// ReadCSV は汎用CSVリーダーです
func (p *CSVProcessor) ReadCSV(filePath string) ([]models.CSVRecord, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("CSVオープンエラー: %w", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, fmt.Errorf("CSV読み込みエラー: %w", err)
    }

    if len(records) < 2 {
        return nil, fmt.Errorf("CSVデータが不足しています")
    }

    headers := records[0]
    result := make([]models.CSVRecord, 0, len(records)-1)

    for _, record := range records[1:] {
        rowData := make(models.CSVRecord)
        for j := 0; j < min(len(headers), len(record)); j++ {
            rowData[headers[j]] = record[j]
        }
        result = append(result, rowData)
    }

    return result, nil
}

// min は２つの整数の小さい方を返します
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

// WriteJiraCSV はJIRA用のCSVを作成します
func (p *CSVProcessor) WriteJiraCSV(records []models.CSVRecord) error {
	utils.LogInfo("JIRA CSVファイル '%s' を作成します", p.config.JiraCSV)

	if len(records) == 0 {
		return fmt.Errorf("書き込むデータがありません")
	}

	file, err := os.Create(p.config.JiraCSV)
	if err != nil {
		return fmt.Errorf("CSVファイル作成エラー: %w", err)
	}
	defer file.Close()

	// 出力するフィールドと順序を定義
	headers := []string{
		"JIRA Issue ID", "Title", "Description", "Labels", "Type",
		"JIRA Status", "Story Points", "Created Date", "Resolved Date",
		"Assignee", "Reporter", "Comment", "JIRA Issue Key",
	}

	writer := csv.NewWriter(file)
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("ヘッダー書き込みエラー: %w", err)
	}

	for _, record := range records {
		row := make([]string, len(headers))
		for i, header := range headers {
			row[i] = record[header]
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("行書き込みエラー: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("CSV書き込み完了エラー: %w", err)
	}

	utils.LogInfo("CSV書き込み完了: %d 行", len(records))
	return nil
}

// LoadIssueMapping はCSVからPivotal ID → JIRA Key のマッピングを読み込みます
func (p *CSVProcessor) LoadIssueMapping() (models.IssueMapping, error) {
	utils.LogInfo("イシューマッピングを読み込んでいます...")

	file, err := os.Open(p.config.JiraCSV)
	if err != nil {
		return nil, fmt.Errorf("マッピングCSVオープンエラー: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("マッピングCSV読み込みエラー: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("マッピングデータが不足しています")
	}

	headers := records[0]
	var idIndex, keyIndex int = -1, -1

	for i, header := range headers {
		if header == "JIRA Issue ID" {
			idIndex = i
		} else if header == "JIRA Issue Key" {
			keyIndex = i
		}
	}

	if idIndex == -1 || keyIndex == -1 {
		return nil, fmt.Errorf("マッピングに必要なカラムが見つかりません")
	}

	mapping := make(models.IssueMapping)
	for _, record := range records[1:] {
		if len(record) <= max(idIndex, keyIndex) {
			continue
		}

		pivotalID := record[idIndex]
		jiraKey := record[keyIndex]
		if pivotalID != "" && jiraKey != "" && jiraKey != "ERROR" {
			mapping[pivotalID] = jiraKey
		}
	}

	utils.LogInfo("イシューマッピングをロードしました: %d 件", len(mapping))
	return mapping, nil
}

// UpdateJiraKeys はCSVファイルのJIRAキーを更新します
func (p *CSVProcessor) UpdateJiraKeys(mapping models.IssueMapping) error {
	utils.LogInfo("JIRAキーをCSVファイルに更新しています...")

	// CSVを読み込む
	file, err := os.Open(p.config.JiraCSV)
	if err != nil {
		return fmt.Errorf("CSVオープンエラー: %w", err)
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	file.Close() // 早めに閉じる

	if err != nil {
		return fmt.Errorf("CSV読み込みエラー: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("更新するデータが不足しています")
	}

	// ヘッダーとカラムインデックスを取得
	headers := records[0]
	var idIndex, keyIndex int = -1, -1

	for i, header := range headers {
		if header == "JIRA Issue ID" {
			idIndex = i
		} else if header == "JIRA Issue Key" {
			keyIndex = i
		}
	}

	if idIndex == -1 || keyIndex == -1 {
		return fmt.Errorf("必要なカラムが見つかりません")
	}

	// マッピングを適用
	updated := 0
	for i, record := range records[1:] {
		if len(record) <= max(idIndex, keyIndex) {
			continue
		}

		pivotalID := record[idIndex]
		if jiraKey, ok := mapping[pivotalID]; ok {
			records[i+1][keyIndex] = jiraKey
			updated++
		}
	}

	// 更新したCSVを書き込む
	outFile, err := os.Create(p.config.JiraCSV)
	if err != nil {
		return fmt.Errorf("CSVファイル作成エラー: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	if err := writer.WriteAll(records); err != nil {
		return fmt.Errorf("CSV書き込みエラー: %w", err)
	}

	utils.LogInfo("JIRAキーの更新完了: %d/%d 件を更新しました", updated, len(records)-1)
	return nil
}

// UpdateJiraKeysWithErrorFlags はCSVファイルのJIRAキーとエラーフラグを更新します
func (p *CSVProcessor) UpdateJiraKeysWithErrorFlags(mapping models.IssueMapping, errorFlags map[string]bool) error {
	utils.LogInfo("JIRAキーとエラーフラグをCSVファイルに更新しています...")

	// CSVを読み込む
	file, err := os.Open(p.config.JiraCSV)
	if err != nil {
		return fmt.Errorf("CSVオープンエラー: %w", err)
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	file.Close() // 早めに閉じる

	if err != nil {
		return fmt.Errorf("CSV読み込みエラー: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("更新するデータが不足しています")
	}

	// ヘッダーの確認と拡張
	headers := records[0]
	var idIndex, keyIndex, errorIndex int = -1, -1, -1

	for i, header := range headers {
		if header == "JIRA Issue ID" {
			idIndex = i
		} else if header == "JIRA Issue Key" {
			keyIndex = i
		} else if header == "Error" {
			errorIndex = i
		}
	}

	if idIndex == -1 || keyIndex == -1 {
		return fmt.Errorf("必要なカラムが見つかりません")
	}

	// Errorカラムがなければ追加
	if errorIndex == -1 {
		headers = append(headers, "Error")
		errorIndex = len(headers) - 1

		// 各行にも空のエラーフィールドを追加
		for i := range records[1:] {
			records[i+1] = append(records[i+1], "")
		}
	}

	// マッピングを適用
	updated := 0
	for i, record := range records[1:] {
		if len(record) <= max(idIndex, keyIndex) {
			continue
		}

		pivotalID := record[idIndex]

		// JIRAキーの更新
		if jiraKey, ok := mapping[pivotalID]; ok {
			records[i+1][keyIndex] = jiraKey
			updated++

			// エラーフラグの更新
			if errorFlag, ok := errorFlags[pivotalID]; ok && errorFlag {
				records[i+1][errorIndex] = "1" // エラーあり
			} else {
				records[i+1][errorIndex] = "0" // エラーなし
			}
		}
	}

	// 更新したCSVを書き込む
	outFile, err := os.Create(p.config.JiraCSV)
	if err != nil {
		return fmt.Errorf("CSVファイル作成エラー: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	if err := writer.WriteAll(records); err != nil {
		return fmt.Errorf("CSV書き込みエラー: %w", err)
	}

	utils.LogInfo("JIRAキーとエラーフラグの更新完了: %d/%d 件を更新しました", updated, len(records)-1)
	return nil
}

// 日付文字列を変換
func (p *CSVProcessor) convertDateFormat(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	formats := []string{
		"2006-01-02T15:04:05",
		"1/2/06 3:04 PM",
		"01/Jan/06 3:04 PM",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		t, err := time.Parse(format, dateStr)
		if err == nil {
			return t.Format("2006-01-02T15:04:05.000+0000")
		}
	}

	utils.LogWarn("日付変換エラー: '%s'", dateStr)
	return ""
}

// max は２つの整数の大きい方を返します
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
