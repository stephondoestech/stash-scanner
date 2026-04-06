package review

import "time"

type ItemType string

const (
	SceneItem   ItemType = "scene"
	GalleryItem ItemType = "gallery"
)

type Candidate struct {
	PerformerID string   `json:"performer_id"`
	Name        string   `json:"name"`
	ImageURL    string   `json:"image_url"`
	Aliases     []string `json:"aliases,omitempty"`
	Score       int      `json:"score"`
	Reasons     []string `json:"reasons"`
}

type QueueItem struct {
	ID           string      `json:"id"`
	Type         ItemType    `json:"type"`
	Title        string      `json:"title"`
	Details      string      `json:"details,omitempty"`
	Path         string      `json:"path,omitempty"`
	Tags         []string    `json:"tags,omitempty"`
	Studio       string      `json:"studio,omitempty"`
	Status       string      `json:"status"`
	BestScore    int         `json:"best_score"`
	CandidateCnt int         `json:"candidate_count"`
	Candidates   []Candidate `json:"candidates,omitempty"`
}

type Snapshot struct {
	RefreshedAt time.Time   `json:"refreshed_at"`
	ItemCount   int         `json:"item_count"`
	ReviewCount int         `json:"review_count"`
	EmptyCount  int         `json:"empty_count"`
	LastError   string      `json:"last_error,omitempty"`
	Items       []QueueItem `json:"items"`
}

type Status struct {
	Now         time.Time   `json:"now"`
	Running     bool        `json:"running"`
	RefreshedAt time.Time   `json:"refreshed_at"`
	ItemCount   int         `json:"item_count"`
	ReviewCount int         `json:"review_count"`
	EmptyCount  int         `json:"empty_count"`
	LastError   string      `json:"last_error,omitempty"`
	Items       []QueueItem `json:"items"`
}
