package chapterplan

import (
	"testing"

	"github.com/google/uuid"
)

func TestCandidateBelongsToBatchRequiresBatchAndProjectMatch(t *testing.T) {
	projectID, batchID := uuid.New(), uuid.New()
	batch := CandidateBatch{ID: batchID, ProjectID: projectID}
	tests := []struct {
		name      string
		candidate Candidate
		want      bool
	}{
		{"same batch and project", Candidate{BatchID: batchID, ProjectID: projectID}, true},
		{"other batch", Candidate{BatchID: uuid.New(), ProjectID: projectID}, false},
		{"other project", Candidate{BatchID: batchID, ProjectID: uuid.New()}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := candidateBelongsToBatch(test.candidate, batch); got != test.want {
				t.Fatalf("candidateBelongsToBatch()=%v, want %v", got, test.want)
			}
		})
	}
}
