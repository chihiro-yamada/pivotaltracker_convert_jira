package models

import "time"

// PivotalTask はPivotalTrackerのタスクを表します
type PivotalTask struct {
	ID           string
	Title        string
	Description  string
	Labels       []string
	Type         string
	Estimate     int
	CurrentState string
	CreatedAt    time.Time
	AcceptedAt   time.Time
	OwnedBy      string
	Comments     string
}

// JiraIssue はJIRAのイシューを表します
type JiraIssue struct {
	ID           string // Pivotal ID (参照用)
	Key          string // JIRA Key (PROJECT-123 形式)
	Title        string
	Description  string
	Labels       []string
	Type         string
	StoryPoints  int
	Status       string
	CreatedDate  time.Time
	ResolvedDate time.Time
	Assignee     string
	Comments     string
}

// CSVRecord はCSVの1行を表します (ヘッダー名→値のマップ)
type CSVRecord map[string]string

// IssueMapping はPivotal IDとJIRAキーのマッピングを表します
type IssueMapping map[string]string
