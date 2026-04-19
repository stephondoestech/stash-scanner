package review

import "time"

type ItemType string

const (
	SceneItem   ItemType = "scene"
	GalleryItem ItemType = "gallery"
)

type ReviewState string

const (
	ReviewPending  ReviewState = "pending"
	ReviewSkipped  ReviewState = "skipped"
	ReviewResolved ReviewState = "resolved"
)

type MatchSettings struct {
	MinScore int `json:"min_score"`
	MinLead  int `json:"min_lead"`
}

type AuditEntry struct {
	At            time.Time `json:"at"`
	Action        string    `json:"action"`
	Detail        string    `json:"detail,omitempty"`
	ItemIDs       []string  `json:"item_ids,omitempty"`
	MatchMinScore int       `json:"match_min_score"`
	MatchMinLead  int       `json:"match_min_lead"`
}

type Candidate struct {
	PerformerID string   `json:"performer_id"`
	Name        string   `json:"name"`
	ImageURL    string   `json:"image_url"`
	Gender      string   `json:"gender,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Incomplete  bool     `json:"incomplete,omitempty"`
	CanRepair   bool     `json:"can_repair,omitempty"`
	Score       int      `json:"score"`
	Reasons     []string `json:"reasons"`
}

type LinkedPerformer struct {
	PerformerID string   `json:"performer_id"`
	Name        string   `json:"name"`
	ImageURL    string   `json:"image_url"`
	Gender      string   `json:"gender,omitempty"`
	Incomplete  bool     `json:"incomplete,omitempty"`
	CanRepair   bool     `json:"can_repair,omitempty"`
	StashIDs    []string `json:"stash_ids,omitempty"`
}

type QueueItem struct {
	ID                   string            `json:"id"`
	Type                 ItemType          `json:"type"`
	Title                string            `json:"title"`
	Details              string            `json:"details,omitempty"`
	Path                 string            `json:"path,omitempty"`
	Tags                 []string          `json:"tags,omitempty"`
	Studio               string            `json:"studio,omitempty"`
	Status               string            `json:"status"`
	SuppressionReason    string            `json:"suppression_reason,omitempty"`
	AutoAssigned         bool              `json:"auto_assigned,omitempty"`
	AutoAssignReason     string            `json:"auto_assign_reason,omitempty"`
	LinkedPerformers     []LinkedPerformer `json:"linked_performers,omitempty"`
	ReviewState          ReviewState       `json:"review_state"`
	ReviewedAt           time.Time         `json:"reviewed_at,omitempty"`
	ResolvedAt           time.Time         `json:"resolved_at,omitempty"`
	AssignedPerformerIDs []string          `json:"assigned_performer_ids,omitempty"`
	BestScore            int               `json:"best_score"`
	CandidateCnt         int               `json:"candidate_count"`
	Candidates           []Candidate       `json:"candidates,omitempty"`
}

type Snapshot struct {
	RefreshedAt       time.Time     `json:"refreshed_at"`
	ItemCount         int           `json:"item_count"`
	ActiveCount       int           `json:"active_count"`
	SkippedCount      int           `json:"skipped_count"`
	ResolvedCount     int           `json:"resolved_count"`
	ReviewCount       int           `json:"review_count"`
	RepairCount       int           `json:"repair_count"`
	AutoAssignedCount int           `json:"auto_assigned_count"`
	SuppressedCount   int           `json:"suppressed_count"`
	EmptyCount        int           `json:"empty_count"`
	Settings          MatchSettings `json:"settings"`
	AuditTrail        []AuditEntry  `json:"audit_trail,omitempty"`
	LastError         string        `json:"last_error,omitempty"`
	Items             []QueueItem   `json:"items"`
}

type Status struct {
	Now               time.Time    `json:"now"`
	Running           bool         `json:"running"`
	RefreshedAt       time.Time    `json:"refreshed_at"`
	ItemCount         int          `json:"item_count"`
	VisibleCount      int          `json:"visible_count"`
	ActiveCount       int          `json:"active_count"`
	SkippedCount      int          `json:"skipped_count"`
	ResolvedCount     int          `json:"resolved_count"`
	ReviewCount       int          `json:"review_count"`
	RepairCount       int          `json:"repair_count"`
	AutoAssignedCount int          `json:"auto_assigned_count"`
	SuppressedCount   int          `json:"suppressed_count"`
	EmptyCount        int          `json:"empty_count"`
	MatchMinScore     int          `json:"match_min_score"`
	MatchMinLead      int          `json:"match_min_lead"`
	FilterQuery       string       `json:"filter_query,omitempty"`
	AuditTrail        []AuditEntry `json:"audit_trail,omitempty"`
	LastError         string       `json:"last_error,omitempty"`
	Items             []QueueItem  `json:"items"`
}
