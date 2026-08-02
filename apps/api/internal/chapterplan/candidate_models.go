package chapterplan

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type BatchTarget struct {
	StartChapterNo        int `json:"startChapterNo"`
	EndChapterNo          int `json:"endChapterNo"`
	RequestedChapterCount int `json:"requestedChapterCount"`
}

type InputSummary struct {
	GenerationMode     string          `json:"generationMode"`
	Target             BatchTarget     `json:"target"`
	StorylineSelection json.RawMessage `json:"storylineSelection"`
	ContextOptions     json.RawMessage `json:"contextOptions"`
}

type CandidateBatch struct {
	ID                      uuid.UUID       `json:"id"`
	ProjectID               uuid.UUID       `json:"projectId"`
	SourceWorkflowRunID     uuid.UUID       `json:"sourceWorkflowRunId"`
	GenerationMode          string          `json:"generationMode"`
	RangeStart              *int            `json:"-"`
	RangeEnd                *int            `json:"-"`
	RequestedChapterCount   int             `json:"-"`
	Target                  BatchTarget     `json:"target"`
	InputDigest             string          `json:"inputDigest"`
	InputSummary            InputSummary    `json:"inputSummary"`
	InputSnapshot           json.RawMessage `json:"-"`
	StorylineSnapshot       json.RawMessage `json:"-"`
	ContextOptions          json.RawMessage `json:"-"`
	AdditionalInstructions  *string         `json:"-"`
	WorkflowBindingSnapshot json.RawMessage `json:"-"`
	CandidateCount          int             `json:"candidateCount"`
	PendingCount            int             `json:"pendingCount"`
	StaleCount              int             `json:"staleCount"`
	AdoptedCount            int             `json:"adoptedCount"`
	DiscardedCount          int             `json:"discardedCount"`
	Status                  string          `json:"status"`
	Version                 int             `json:"version"`
	CompletedAt             *time.Time      `json:"completedAt"`
	AbandonedAt             *time.Time      `json:"abandonedAt"`
	AbandonReason           *string         `json:"-"`
	CreatedBy               string          `json:"-"`
	UpdatedBy               string          `json:"-"`
	CreatedAt               time.Time       `json:"createdAt"`
	UpdatedAt               time.Time       `json:"updatedAt"`
}

type Candidate struct {
	ID                     uuid.UUID       `json:"id"`
	BatchID                uuid.UUID       `json:"batchId"`
	ProjectID              uuid.UUID       `json:"projectId"`
	ChapterNo              int             `json:"chapterNo"`
	SortOrder              int             `json:"sortOrder"`
	BaseChapterPlanID      *uuid.UUID      `json:"baseChapterPlanId"`
	BaseRevisionID         *uuid.UUID      `json:"baseRevisionId"`
	BaseChapterVersion     *int            `json:"-"`
	BaseChapterPlanVersion *int            `json:"baseChapterPlanVersion"`
	BaseSnapshot           json.RawMessage `json:"baseSnapshot"`
	GeneratedSnapshot      json.RawMessage `json:"generatedSnapshot"`
	CurrentSnapshot        json.RawMessage `json:"currentSnapshot"`
	DiffType               string          `json:"diffType"`
	Status                 string          `json:"status"`
	AdoptedChapterPlanID   *uuid.UUID      `json:"-"`
	AdoptedRevisionID      *uuid.UUID      `json:"-"`
	AdoptedAt              *time.Time      `json:"-"`
	DiscardedAt            *time.Time      `json:"-"`
	DiscardReason          *string         `json:"-"`
	CreatedBy              string          `json:"-"`
	UpdatedBy              string          `json:"-"`
	LastEditedBy           *string         `json:"-"`
	LastEditedAt           *time.Time      `json:"-"`
	Version                int             `json:"version"`
	CreatedAt              time.Time       `json:"createdAt"`
	UpdatedAt              time.Time       `json:"updatedAt"`
}

type Revision struct {
	ID                     uuid.UUID       `json:"id"`
	ChapterPlanID          uuid.UUID       `json:"chapterPlanId"`
	ProjectID              uuid.UUID       `json:"projectId"`
	RevisionNo             int             `json:"revisionNo"`
	Snapshot               json.RawMessage `json:"snapshot"`
	ChangeType             string          `json:"changeType"`
	SourceCandidateID      *uuid.UUID      `json:"sourceCandidateId"`
	SourceCandidateBatchID *uuid.UUID      `json:"sourceCandidateBatchId"`
	SourceWorkflowRunID    *uuid.UUID      `json:"sourceWorkflowRunId"`
	CreatedBy              string          `json:"-"`
	CreatedAt              time.Time       `json:"createdAt"`
}

type BatchFilter struct {
	Status              *string
	GenerationMode      *string
	SourceWorkflowRunID *uuid.UUID
	CreatedAtFrom       *time.Time
	CreatedAtTo         *time.Time
	Limit               int
	Offset              int
}

type CandidateFilter struct {
	Status      *string
	DiffType    *string
	StorylineID *uuid.UUID
	Q           *string
	Limit       int
	Offset      int
}

type BatchListResult struct {
	Items  []CandidateBatch `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type CandidateListResult struct {
	Items  []Candidate `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type RevisionListResult struct {
	Items  []Revision `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

type CandidateBatchCounts struct {
	Ready            int `json:"ready"`
	PartiallyAdopted int `json:"partiallyAdopted"`
	Adopted          int `json:"adopted"`
	Abandoned        int `json:"abandoned"`
}

type Summary struct {
	CurrentChapterCount      int                  `json:"currentChapterCount"`
	PendingConfirmationCount int                  `json:"pendingConfirmationCount"`
	ConfirmedChapterCount    int                  `json:"confirmedChapterCount"`
	CandidateBatchCounts     CandidateBatchCounts `json:"candidateBatchCounts"`
	ActiveRun                json.RawMessage      `json:"activeRun"`
}

type RuntimeResultError struct {
	Cause      error
	RunID      uuid.UUID
	RunVersion int
}

func (e *RuntimeResultError) Error() string { return e.Cause.Error() }
func (e *RuntimeResultError) Unwrap() error { return e.Cause }
