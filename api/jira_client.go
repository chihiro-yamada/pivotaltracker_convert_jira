package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pivotaltojira/config"
	"pivotaltojira/utils"
)

// JiraClient はJIRA APIとのやり取りを処理します
type JiraClient struct {
	config *config.Config
	client *http.Client
}

// NewJiraClient は新しいJIRAクライアントを作成します
func NewJiraClient(cfg *config.Config) *JiraClient {
	return &JiraClient{
		config: cfg,
		client: &http.Client{},
	}
}

// CheckAuth はJIRA認証をチェックします
func (j *JiraClient) CheckAuth() error {
	url := fmt.Sprintf("%s/rest/api/2/myself", j.config.JiraURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	req.SetBasicAuth(j.config.JiraEmail, j.config.JiraAPIToken)

	resp, err := j.retryOnRateLimit(req)
	if err != nil {
		return fmt.Errorf("リクエスト送信エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("認証失敗: %s", string(body))
	}

	return nil
}

// CreateIssue はJIRAイシューを作成します
func (j *JiraClient) CreateIssue(summary, description string, labels []string, issueType string, reporter string, assignee string) (string, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue", j.config.JiraURL)

	// サマリーから改行文字を削除
	summary = strings.ReplaceAll(summary, "\n", " ")
	summary = strings.ReplaceAll(summary, "\r", " ")

	// 連続する空白を単一の空白に置換
	summary = strings.Join(strings.Fields(summary), " ")

	// ラベルが空でないことを確認
	if labels == nil {
		labels = []string{}
	}

	// フィールドの作成
	fields := map[string]interface{}{
		"project":     map[string]string{"key": j.config.JiraProjectKey},
		"summary":     summary,
		"description": description,
		"issuetype":   map[string]string{"name": issueType},
		"labels":      labels,
	}

	//　担当者と報告者が指定されている場合のマッピング対応
	j.prepareUserFields(fields, assignee, reporter, description)

	// ペイロードの作成
	payload := map[string]interface{}{
		"fields": fields,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("JSONエンコードエラー: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	req.SetBasicAuth(j.config.JiraEmail, j.config.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.retryOnRateLimit(req)
	if err != nil {
		return "", fmt.Errorf("リクエスト送信エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("イシュー作成失敗: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("レスポンス解析エラー: %w", err)
	}

	issueKey, ok := result["key"].(string)
	if !ok {
		return "", fmt.Errorf("イシューキーが見つかりません")
	}

	return issueKey, nil
}

// UpdateStoryPoints はJIRAイシューのストーリーポイントを更新します
func (j *JiraClient) UpdateStoryPoints(issueKey string, storyPoints int) error {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s", j.config.JiraURL, issueKey)

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			j.config.StoryPointField: storyPoints,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSONエンコードエラー: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	req.SetBasicAuth(j.config.JiraEmail, j.config.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.retryOnRateLimit(req)
	if err != nil {
		return fmt.Errorf("リクエスト送信エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ストーリーポイント更新失敗: %s", string(body))
	}

	return nil
}

// GetTransitions はイシューの利用可能なトランジションを取得します
func (j *JiraClient) GetTransitions(issueKey string) (map[string]string, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", j.config.JiraURL, issueKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	req.SetBasicAuth(j.config.JiraEmail, j.config.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.retryOnRateLimit(req)
	if err != nil {
		return nil, fmt.Errorf("リクエスト送信エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("トランジション取得失敗: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("レスポンス解析エラー: %w", err)
	}

	transitions, ok := result["transitions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("トランジションが見つかりません")
	}

	transitionMap := make(map[string]string)

	for _, t := range transitions {
		transition, ok := t.(map[string]interface{})
		if !ok {
			continue
		}

		id, ok := transition["id"].(string)
		if !ok {
			continue
		}

		to, ok := transition["to"].(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := to["name"].(string)
		if !ok {
			continue
		}

		transitionMap[strings.ToLower(name)] = id
	}

	return transitionMap, nil
}

// UpdateStatus はJIRAイシューのステータスを更新します
func (j *JiraClient) UpdateStatus(issueKey, targetStatus string) error {
	if strings.ToLower(targetStatus) == "backlog" {
		utils.LogInfo("イシュー %s: 'Backlog' ステータスはスキップします", issueKey)
		return nil // Backlogステータスはスキップ
	}

	transitions, err := j.GetTransitions(issueKey)
	if err != nil {
		return err
	}

	transitionID, ok := transitions[strings.ToLower(targetStatus)]
	if !ok {
		return fmt.Errorf("ステータス '%s' への遷移が見つかりません", targetStatus)
	}

	url := fmt.Sprintf("%s/rest/api/2/issue/%s/transitions", j.config.JiraURL, issueKey)

	payload := map[string]interface{}{
		"transition": map[string]string{
			"id": transitionID,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSONエンコードエラー: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	req.SetBasicAuth(j.config.JiraEmail, j.config.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.retryOnRateLimit(req)
	if err != nil {
		return fmt.Errorf("リクエスト送信エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ステータス更新失敗: %s", string(body))
	}

	return nil
}

// prepareUserFields はユーザーマッピングを処理し、フィールドマップを更新します
func (j *JiraClient) prepareUserFields(fields map[string]interface{}, assignee, reporter, description string) {
	// ユーザー名からJIRAアカウントIDへのマッピング
	userMapping := map[string]string{
		"pivotal_user1": "jira_user1",
		// 必要に応じて追加
	}

	// 現在の説明文
	currentDesc := description

	// 担当者の設定
	if assignee != "" {
		if accountId, ok := userMapping[assignee]; ok {
			fields["assignee"] = map[string]string{"id": accountId}
		} else {
			// マッピングにない場合は説明文に追記
			currentDesc += fmt.Sprintf("\n\n担当者: %s", assignee)
		}
	}

	// 報告者の設定
	if reporter != "" {
		if accountId, ok := userMapping[reporter]; ok {
			fields["reporter"] = map[string]string{"id": accountId}
		} else {
			// マッピングにない場合は説明文に追記
			currentDesc += fmt.Sprintf("\n\n報告者: %s", reporter)
		}
	}

	// 説明文が更新された場合のみ設定
	if currentDesc != description {
		fields["description"] = currentDesc
	}
}

// AddComment はJIRAイシューにコメントを追加します
func (j *JiraClient) AddComment(issueKey, comment string) error {
	// コメントが空の場合は何もしない
	if comment == "" {
		return nil
	}

	url := fmt.Sprintf("%s/rest/api/2/issue/%s/comment", j.config.JiraURL, issueKey)

	// ペイロードの作成
	payload := map[string]string{
		"body": comment,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSONエンコードエラー: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	req.SetBasicAuth(j.config.JiraEmail, j.config.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := j.retryOnRateLimit(req)
	if err != nil {
		return fmt.Errorf("リクエスト送信エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("コメント追加失敗: %s", string(body))
	}

	return nil
}

// UploadAttachment はJIRAイシューに添付ファイルをアップロードします
func (j *JiraClient) UploadAttachment(issueKey, filePath string) error {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/attachments", j.config.JiraURL, issueKey)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("ファイルオープンエラー: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("multipartフォーム作成エラー: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("ファイルコピーエラー: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("writerクローズエラー: %w", err)
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("リクエスト作成エラー: %w", err)
	}

	req.SetBasicAuth(j.config.JiraEmail, j.config.JiraAPIToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Atlassian-Token", "no-check")

	resp, err := j.retryOnRateLimit(req)
	if err != nil {
		return fmt.Errorf("リクエスト送信エラー: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("添付ファイルアップロード失敗: %s", string(bodyBytes))
	}

	return nil
}

// retryOnRateLimit はレート制限エラー(429)の場合に10秒待機して再試行します
func (j *JiraClient) retryOnRateLimit(req *http.Request) (*http.Response, error) {
    // 最初の試行
    resp, err := j.client.Do(req)
    if err != nil {
        return nil, err
    }

    // 429（レート制限）でなければそのまま返す
    if resp.StatusCode != 429 {
        return resp, nil
    }

    // レート制限エラーの場合、レスポンスボディを読んでクローズ
    body, _ := io.ReadAll(resp.Body)
    resp.Body.Close()

    // 10秒待機して再試行
    utils.LogWarn("レート制限に達しました。10秒後に再試行します。エラー: %s", string(body))
    time.Sleep(10 * time.Second)

    // リクエストのボディを再設定（必要な場合）
    if req.Body != nil {
        bodyBytes, _ := io.ReadAll(req.Body)
        req.Body.Close()
        req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
    }

    // 再試行
    return j.client.Do(req)
}
