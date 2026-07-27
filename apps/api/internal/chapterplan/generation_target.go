package chapterplan

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidGenerationTarget = errors.New("invalid chapter planning generation target")
var ErrPreflightTokenInvalid = errors.New("preflight token invalid")
var ErrPreflightTokenExpired = errors.New("preflight token expired")

type GenerationTargetRequest struct {
	TargetTotalChapters int `json:"targetTotalChapters"`
	ChapterCount        int `json:"chapterCount"`
	StartChapterNo      int `json:"startChapterNo"`
	EndChapterNo        int `json:"endChapterNo"`
}

// NormalizeGenerationTarget applies the frozen full, append, and range rules before
// any workflow or candidate write is attempted.
func NormalizeGenerationTarget(mode string, target GenerationTargetRequest, currentMaxChapter int) (BatchTarget, error) {
	if currentMaxChapter < 0 || currentMaxChapter > 100 {
		return BatchTarget{}, ErrInvalidGenerationTarget
	}
	switch mode {
	case "full":
		if target.TargetTotalChapters < 1 || target.TargetTotalChapters > 100 {
			return BatchTarget{}, ErrInvalidGenerationTarget
		}
		return BatchTarget{StartChapterNo: 1, EndChapterNo: target.TargetTotalChapters, RequestedChapterCount: target.TargetTotalChapters}, nil
	case "append":
		if target.ChapterCount < 1 || target.ChapterCount > 100-currentMaxChapter {
			return BatchTarget{}, ErrInvalidGenerationTarget
		}
		start := currentMaxChapter + 1
		return BatchTarget{StartChapterNo: start, EndChapterNo: start + target.ChapterCount - 1, RequestedChapterCount: target.ChapterCount}, nil
	case "range":
		if target.StartChapterNo < 1 || target.EndChapterNo < target.StartChapterNo || target.EndChapterNo > 100 {
			return BatchTarget{}, ErrInvalidGenerationTarget
		}
		return BatchTarget{StartChapterNo: target.StartChapterNo, EndChapterNo: target.EndChapterNo, RequestedChapterCount: target.EndChapterNo - target.StartChapterNo + 1}, nil
	default:
		return BatchTarget{}, ErrInvalidGenerationTarget
	}
}

type PreflightTokenClaims struct {
	ProjectID              uuid.UUID       `json:"projectId"`
	ActorID                string          `json:"actorId"`
	Stage                  string          `json:"stage"`
	GenerationMode         string          `json:"generationMode,omitempty"`
	StorylineSelectionMode string          `json:"storylineSelectionMode,omitempty"`
	StorylineIDs           []uuid.UUID     `json:"storylineIds,omitempty"`
	ContextOptions         json.RawMessage `json:"contextOptions,omitempty"`
	AdditionalInstructions *string         `json:"additionalInstructions,omitempty"`
	Target                 BatchTarget     `json:"target"`
	InputDigest            string          `json:"inputDigest"`
	BindingID              uuid.UUID       `json:"bindingId"`
	BindingVersion         int             `json:"bindingVersion"`
	IssuedAt               int64           `json:"iat"`
	ExpiresAt              int64           `json:"exp"`
	Nonce                  string          `json:"jti"`
}

func SignPreflightToken(secret []byte, claims PreflightTokenClaims) (string, error) {
	if len(secret) == 0 || claims.ProjectID == uuid.Nil || claims.ActorID == "" || claims.Stage != "chapter_planning" || !digestPattern.MatchString(claims.InputDigest) || claims.BindingID == uuid.Nil || claims.BindingVersion < 1 {
		return "", ErrPreflightTokenInvalid
	}
	if claims.ExpiresAt != claims.IssuedAt+int64((10*time.Minute).Seconds()) || claims.ExpiresAt <= time.Now().Unix() {
		return "", ErrPreflightTokenInvalid
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal preflight token: %w", err)
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyPreflightToken(secret []byte, raw string, now time.Time) (PreflightTokenClaims, error) {
	var claims PreflightTokenClaims
	parts := splitToken(raw)
	if len(secret) == 0 || len(parts) != 2 {
		return claims, ErrPreflightTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, ErrPreflightTokenInvalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, ErrPreflightTokenInvalid
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return claims, ErrPreflightTokenInvalid
	}
	if json.Unmarshal(payload, &claims) != nil {
		return claims, ErrPreflightTokenInvalid
	}
	if claims.ExpiresAt <= now.Unix() {
		return claims, ErrPreflightTokenExpired
	}
	if claims.Stage != "chapter_planning" || claims.ProjectID == uuid.Nil || claims.ActorID == "" || claims.BindingID == uuid.Nil || claims.BindingVersion < 1 || !digestPattern.MatchString(claims.InputDigest) {
		return claims, ErrPreflightTokenInvalid
	}
	return claims, nil
}
func splitToken(value string) []string {
	for i := range value {
		if value[i] == '.' {
			return []string{value[:i], value[i+1:]}
		}
	}
	return nil
}
