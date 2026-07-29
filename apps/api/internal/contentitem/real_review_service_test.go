package contentitem

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const validReviewOutput = `{
	"schemaVersion":"review.output.v1",
	"conclusion":"needs_changes",
	"summary":"需要调整",
	"passedRuleCount":4,
	"issues":[{
		"issueKey":"character-consistency-1",
		"position":1,
		"categoryKey":"character_consistency",
		"categoryLabel":"角色一致性",
		"severity":"warning",
		"title":"人物行为与设定不一致",
		"description":"问题说明",
		"evidence":{"quote":"必要短引文","sourceRefs":["sourceContentVersion"]},
		"location":{"paragraphStart":1,"paragraphEnd":1,"sentenceStart":1,"sentenceEnd":1},
		"suggestion":"修改建议"
	}],
	"recommendations":[{
		"position":1,
		"priority":"medium",
		"title":"报告级建议",
		"description":"建议说明"
	}]
}`

func TestDecodeReviewRuntimeOutputStrictContract(t *testing.T) {
	output, err := DecodeReviewRuntimeOutput(json.RawMessage(validReviewOutput))
	if err != nil {
		t.Fatal(err)
	}
	if output.SchemaVersion != "review.output.v1" || output.Conclusion != "needs_changes" ||
		len(output.Issues) != 1 || len(output.Recommendations) != 1 {
		t.Fatalf("output=%+v", output)
	}
}

func TestDecodeReviewRuntimeOutputRejectsFrozenInvalidShapes(t *testing.T) {
	longSummary, _ := json.Marshal(strings.Repeat("界", 5001))
	cases := map[string]string{
		"missing required": `{"schemaVersion":"review.output.v1","conclusion":"passed"}`,
		"unknown field": strings.Replace(validReviewOutput, `"passedRuleCount":4,`, `"passedRuleCount":4,"credentials":"secret",`, 1),
		"trailing json": validReviewOutput + `{}`,
		"invalid conclusion": strings.Replace(validReviewOutput, `"needs_changes"`, `"pass"`, 1),
		"invalid severity": strings.Replace(validReviewOutput, `"warning"`, `"high"`, 1),
		"invalid category": strings.Replace(validReviewOutput, `"character_consistency"`, `"Character Consistency"`, 1),
		"duplicate issue key": strings.Replace(validReviewOutput, `"recommendations":[`, `{"issueKey":"character-consistency-1","position":2,"categoryKey":"language_quality","categoryLabel":"语言质量","severity":"suggestion","title":"标题","description":"说明","evidence":{"quote":null,"sourceRefs":[]},"location":null,"suggestion":null}],"recommendations":[`, 1),
		"duplicate position": strings.Replace(validReviewOutput, `"recommendations":[`, `{"issueKey":"language-2","position":1,"categoryKey":"language_quality","categoryLabel":"语言质量","severity":"suggestion","title":"标题","description":"说明","evidence":{"quote":null,"sourceRefs":[]},"location":null,"suggestion":null}],"recommendations":[`, 1),
		"non contiguous position": strings.Replace(validReviewOutput, `"position":1,`, `"position":2,`, 1),
		"invalid location": strings.Replace(validReviewOutput, `"paragraphStart":1,"paragraphEnd":1`, `"paragraphStart":2,"paragraphEnd":1`, 1),
		"duplicate json key": strings.Replace(validReviewOutput, `"summary":"需要调整",`, `"summary":"需要调整","summary":"重复",`, 1),
		"oversize text": strings.Replace(validReviewOutput, `"summary":"需要调整"`, `"summary":`+string(longSummary), 1),
		"needs changes without issue": `{"schemaVersion":"review.output.v1","conclusion":"needs_changes","summary":"需要调整","passedRuleCount":1,"issues":[],"recommendations":[]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReviewRuntimeOutput(json.RawMessage(raw)); !errors.Is(err, ErrReviewOutputInvalid) {
				t.Fatalf("error=%v raw=%s", err, raw)
			}
		})
	}
}

func TestDecodeReviewRuntimeOutputAcceptsPassedWithoutIssues(t *testing.T) {
	raw := `{"schemaVersion":"review.output.v1","conclusion":"passed","summary":"审核通过","passedRuleCount":8,"issues":[],"recommendations":[]}`
	if _, err := DecodeReviewRuntimeOutput(json.RawMessage(raw)); err != nil {
		t.Fatal(err)
	}
}
