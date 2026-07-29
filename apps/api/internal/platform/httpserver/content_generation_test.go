package httpserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
)

func TestContentGenerationInfrastructureErrorUsesSafeInternalEnvelope(t *testing.T) {
	response:=httptest.NewRecorder()
	request:=httptest.NewRequest("POST","/api/v1/content-generation-runs/test/result-consumption-retries",nil)
	contentGenerationError(response,request,errors.New("postgres source query failed"))
	body:=response.Body.String()
	if response.Code!=500||!strings.Contains(body,`"code":"internal_error"`)||strings.Contains(body,"postgres")||strings.Contains(body,"result_consumption_failed"){t.Fatalf("status=%d body=%s",response.Code,body)}
}

func TestRewriteSetCurrentErrorsUseFrozenCodes(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{
		{contentitem.ErrRewriteCandidateNotFound, 404, "rewrite_candidate_not_found"},
		{contentitem.ErrRewriteCandidateNotReady, 409, "rewrite_candidate_not_ready"},
		{contentitem.ErrRewriteContentVersionConflict, 409, "content_version_conflict"},
		{contentitem.ErrRewriteIdempotencyConflict, 409, "idempotency_conflict"},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/content-items/test/current-version", nil)
		contentGenerationError(response, request, test.err)
		if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("err=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}
}
