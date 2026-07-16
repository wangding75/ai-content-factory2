package contentitem

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestRewriteContentVersionCreateErrorUsesExactConstraint(t *testing.T) {
	duplicateRewrite := ContentVersion{Source: ContentVersionSourceMockRewrite, VersionNo: 2}
	if got := rewriteContentVersionCreateError(&pgconn.PgError{Code: "23505", ConstraintName: contentVersionItemVersionNoUniqueConstraint}, duplicateRewrite); !errors.Is(got, ErrRewriteAlreadyExists) {
		t.Fatalf("got %v", got)
	}
	for _, tc := range []struct {
		name  string
		err   error
		value ContentVersion
	}{
		{"other constraint", &pgconn.PgError{Code: "23505", ConstraintName: "content_versions_item_id_id_unique"}, duplicateRewrite},
		{"other version", &pgconn.PgError{Code: "23505", ConstraintName: contentVersionItemVersionNoUniqueConstraint}, ContentVersion{Source: ContentVersionSourceManualCreated, VersionNo: 1}},
		{"unknown", errors.New("private database detail"), duplicateRewrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := rewriteContentVersionCreateError(tc.err, tc.value); errors.Is(got, ErrRewriteAlreadyExists) {
				t.Fatalf("incorrect mapping: %v", got)
			}
		})
	}
}
