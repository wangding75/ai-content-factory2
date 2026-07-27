package chapterplan

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNormalizeGenerationTarget(t *testing.T) {
	full, err := NormalizeGenerationTarget("full", GenerationTargetRequest{TargetTotalChapters: 3}, 2)
	if err != nil || full.StartChapterNo != 1 || full.EndChapterNo != 3 || full.RequestedChapterCount != 3 {
		t.Fatalf("full=%+v err=%v", full, err)
	}
	appendTarget, err := NormalizeGenerationTarget("append", GenerationTargetRequest{ChapterCount: 2}, 3)
	if err != nil || appendTarget.StartChapterNo != 4 || appendTarget.EndChapterNo != 5 {
		t.Fatalf("append=%+v err=%v", appendTarget, err)
	}
	rangeTarget, err := NormalizeGenerationTarget("range", GenerationTargetRequest{StartChapterNo: 2, EndChapterNo: 4}, 0)
	if err != nil || rangeTarget.RequestedChapterCount != 3 {
		t.Fatalf("range=%+v err=%v", rangeTarget, err)
	}
	if _, err := NormalizeGenerationTarget("range", GenerationTargetRequest{StartChapterNo: 4, EndChapterNo: 2}, 0); !errors.Is(err, ErrInvalidGenerationTarget) {
		t.Fatalf("invalid range err=%v", err)
	}
}

func TestPreflightTokenRejectsTamperAndExpiry(t *testing.T) {
	now := time.Now().UTC()
	secret := []byte("injected-test-secret")
	claims := PreflightTokenClaims{ProjectID: uuid.New(), ActorID: "actor", Stage: "chapter_planning", Target: BatchTarget{StartChapterNo: 1, EndChapterNo: 1, RequestedChapterCount: 1}, InputDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", BindingID: uuid.New(), BindingVersion: 1, IssuedAt: now.Unix(), ExpiresAt: now.Add(10 * time.Minute).Unix(), Nonce: "nonce"}
	token, err := SignPreflightToken(secret, claims)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyPreflightToken(secret, token, now)
	if err != nil || got.ProjectID != claims.ProjectID {
		t.Fatalf("verify=%+v err=%v", got, err)
	}
	if _, err = VerifyPreflightToken(secret, token+"x", now); !errors.Is(err, ErrPreflightTokenInvalid) {
		t.Fatalf("tamper err=%v", err)
	}
	if _, err = VerifyPreflightToken(secret, token, now.Add(11*time.Minute)); !errors.Is(err, ErrPreflightTokenExpired) {
		t.Fatalf("expiry err=%v", err)
	}
}
