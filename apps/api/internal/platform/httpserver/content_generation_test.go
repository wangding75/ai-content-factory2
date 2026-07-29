package httpserver

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContentGenerationInfrastructureErrorUsesSafeInternalEnvelope(t *testing.T) {
	response:=httptest.NewRecorder()
	request:=httptest.NewRequest("POST","/api/v1/content-generation-runs/test/result-consumption-retries",nil)
	contentGenerationError(response,request,errors.New("postgres source query failed"))
	body:=response.Body.String()
	if response.Code!=500||!strings.Contains(body,`"code":"internal_error"`)||strings.Contains(body,"postgres")||strings.Contains(body,"result_consumption_failed"){t.Fatalf("status=%d body=%s",response.Code,body)}
}
