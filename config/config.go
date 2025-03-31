package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config はアプリケーション全体の設定を保持します
type Config struct {
	// JIRA API設定
	JiraURL         string
	JiraEmail       string
	JiraAPIToken    string
	JiraProjectKey  string
	StoryPointField string

	// ファイルパス
	PivotalCSV        string
	JiraCSV           string
	AttachmentsFolder string

	// 並列処理設定
	MaxConcurrent int
}

// StatusMapping はPivotalステータスからJIRAステータスへのマッピングです
var StatusMapping = map[string]string{
	"unscheduled": "Backlog",
	"unstarted":   "Backlog",
	"started":     "進行中",
	"finished":    "REVIEWS",
	"delivered":   "RELEASED",
	"accepted":    "受け入れ済み",
	"rejected":    "Backlog",
}

// LoadConfig は環境変数から設定を読み込みます
func LoadConfig() (*Config, error) {
	// .envファイルを読み込む
	_ = godotenv.Load()

	config := &Config{
		JiraURL:          strings.TrimRight(os.Getenv("JIRA_URL"), "/"),
		JiraEmail:        os.Getenv("JIRA_EMAIL"),
		JiraAPIToken:     os.Getenv("JIRA_API_TOKEN"),
		JiraProjectKey:   os.Getenv("JIRA_PROJECT_KEY"),
		StoryPointField:  getEnvWithDefault("JIRA_STORY_POINT_FIELD", "customfield_10016"),
		PivotalCSV:       getEnvWithDefault("PIVOTAL_CSV", "pivotal.csv"),
		JiraCSV:          getEnvWithDefault("JIRA_CSV", "jira_import_ready.csv"),
		AttachmentsFolder: getEnvWithDefault("ATTACHMENTS_FOLDER", "attachments"),
		MaxConcurrent:    getEnvAsIntWithDefault("MAX_CONCURRENT", 10),
	}

	return config, nil
}

// デフォルト値付きで環境変数を取得
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// デフォルト値付きで環境変数を整数として取得
func getEnvAsIntWithDefault(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}
