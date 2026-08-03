package workflowrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxWorkflowResponseBytes = 2 << 20

type N8NWorkflowExecutor struct {
	client *http.Client
	credential func(context.Context, uuid.UUID) (string, error)
}

func NewN8NWorkflowExecutor(client *http.Client, credential ...func(context.Context, uuid.UUID) (string, error)) *N8NWorkflowExecutor {
	if client == nil {
		client = &http.Client{}
	}
	e := &N8NWorkflowExecutor{client: client}
	if len(credential) == 1 { e.credential = credential[0] }
	return e
}

func (e *N8NWorkflowExecutor) Verify(context.Context, ExecutionRequest) error {
	return nil
}

func (e *N8NWorkflowExecutor) Execute(ctx context.Context, request ExecutionRequest) (ExecutionResult, error) {
	endpoint, timeout, err := n8nExecutionEndpoint(request.ConfigurationSnapshot)
	if err != nil {
		return ExecutionResult{}, err
	}
	body, err := json.Marshal(struct {
		RunID     string          `json:"runId"`
		ProjectID string          `json:"projectId"`
		Stage     string          `json:"stage"`
		Input     json.RawMessage `json:"input"`
	}{
		RunID: request.RunID.String(), ProjectID: request.ProjectID.String(),
		Stage: request.Stage, Input: request.Input,
	})
	if err != nil {
		return ExecutionResult{}, err
	}
	executionContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(executionContext, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ExecutionResult{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(httpRequest)
	if err != nil {
		if errors.Is(executionContext.Err(), context.DeadlineExceeded) {
			return ExecutionResult{}, ErrExecutionTimeout
		}
		return ExecutionResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return ExecutionResult{}, ErrExecutorUnavailable
	}
	output, err := io.ReadAll(io.LimitReader(response.Body, maxWorkflowResponseBytes+1))
	if err != nil || len(output) > maxWorkflowResponseBytes || !validJSONObject(output) {
		return ExecutionResult{Status: ExecutionFailed, ErrorCode: "invalid_response", ErrorMessage: "workflow execution failed"}, nil
	}
	externalExecutionID := strings.TrimSpace(response.Header.Get("X-N8N-Execution-Id"))
	var acknowledgement struct {
		Accepted bool `json:"accepted"`
	}
	if json.Unmarshal(output, &acknowledgement) == nil && acknowledgement.Accepted {
		if externalExecutionID == "" {
			return ExecutionResult{Status: ExecutionFailed, ErrorCode: "invalid_response", ErrorMessage: "workflow execution failed"}, nil
		}
		return ExecutionResult{Status: ExecutionRunning, ExternalExecutionID: externalExecutionID, Metadata: map[string]string{}}, nil
	}
	return ExecutionResult{
		Status: ExecutionSucceeded, Output: json.RawMessage(output),
		ExternalExecutionID: externalExecutionID,
		Metadata:            map[string]string{},
	}, nil
}

func (e *N8NWorkflowExecutor) Cancel(ctx context.Context, request ExecutionRequest) (ExecutionResult, error) {
	if strings.TrimSpace(request.ExternalExecutionID) == "" {
		return ExecutionResult{}, ErrExecutorUnavailable
	}
	// n8n has no execution-stop public API in the pinned version. The running
	// workflow observes ACF's cancellation endpoint at its own checkpoints.
	// Acknowledging the request here leaves the durable run in cancelling until
	// the next public execution query reports its terminal result.
	return ExecutionResult{Status: ExecutionAccepted, ExternalExecutionID: request.ExternalExecutionID}, nil
}

func (e *N8NWorkflowExecutor) Query(ctx context.Context, request ExecutionRequest) (ExecutionResult, error) {
	return e.executionControl(ctx, request, http.MethodGet)
}

func (e *N8NWorkflowExecutor) executionControl(ctx context.Context, request ExecutionRequest, method string) (ExecutionResult, error) {
	if strings.TrimSpace(request.ExternalExecutionID) == "" {
		return ExecutionResult{}, ErrExecutorUnavailable
	}
	base, timeout, err := n8nBaseEndpoint(request.ConfigurationSnapshot)
	if err != nil {
		return ExecutionResult{}, err
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return ExecutionResult{}, ErrExecutorUnavailable
	}
	endpoint.Path = path.Join(endpoint.Path, "api", "v1", "executions", request.ExternalExecutionID)
	endpoint.RawQuery = "includeData=true"
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestCtx, method, endpoint.String(), nil)
	if err != nil {
		return ExecutionResult{}, err
	}
	credential, err := e.runtimeCredential(requestCtx, request.WorkflowConnectionID)
	if err != nil { return ExecutionResult{}, ErrExecutorUnavailable }
	if credential != "" { httpRequest.Header.Set("X-N8N-API-KEY", credential) }
	response, err := e.client.Do(httpRequest)
	if err != nil {
		if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return ExecutionResult{}, ErrExecutionTimeout
		}
		return ExecutionResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return ExecutionResult{}, ErrExecutionNotFound
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return ExecutionResult{}, ErrExecutorUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxWorkflowResponseBytes+1))
	if err != nil || len(body) > maxWorkflowResponseBytes {
		return ExecutionResult{}, ErrInvalidExecutionResult
	}
	return parseN8NQueryResponse(body, request.ExternalExecutionID)
}

func (e *N8NWorkflowExecutor) runtimeCredential(ctx context.Context, id uuid.UUID) (string, error) {
	if e.credential == nil { return "", nil }
	if id == uuid.Nil { return "", ErrExecutorUnavailable }
	return e.credential(ctx, id)
}

// parseN8NQueryResponse accepts only the small n8n execution representation
// required for restart recovery. It intentionally does not try to normalize
// arbitrary n8n API versions or preserve an upstream response body.
func parseN8NQueryResponse(body json.RawMessage, externalExecutionID string) (ExecutionResult, error) {
	var response struct {
		Status string `json:"status"`
		Data struct { ResultData struct { LastNodeExecuted string `json:"lastNodeExecuted"`; RunData map[string][]struct { Data struct { Main [][]struct { JSON json.RawMessage `json:"json"` } `json:"main"` } `json:"data"` } `json:"runData"` } `json:"resultData"` } `json:"data"`
	}
	if !validJSONObject(body) || json.Unmarshal(body, &response) != nil { return ExecutionResult{}, ErrInvalidExecutionResult }
	result := ExecutionResult{ExternalExecutionID: externalExecutionID}
	switch strings.ToLower(strings.TrimSpace(response.Status)) {
	case "new", "running", "waiting":
		result.Status = ExecutionRunning
		return result, nil
	case "error", "crashed":
		result.Status, result.ErrorCode, result.ErrorMessage = ExecutionFailed, "upstream_execution_failed", "workflow execution failed"
		return result, nil
	case "canceled", "cancelled":
		result.Status = ExecutionCancelled
		return result, nil
	case "success":
	default:
		return ExecutionResult{}, ErrInvalidExecutionResult
	}
	runs := response.Data.ResultData.RunData[response.Data.ResultData.LastNodeExecuted]
	if response.Data.ResultData.LastNodeExecuted == "" || len(runs) == 0 || len(runs[len(runs)-1].Data.Main) == 0 || len(runs[len(runs)-1].Data.Main[0]) == 0 { return ExecutionResult{}, ErrInvalidExecutionResult }
	output := runs[len(runs)-1].Data.Main[0][0].JSON
	var cancellation struct { Status string `json:"status"` }
	if json.Unmarshal(output, &cancellation) == nil && strings.EqualFold(cancellation.Status, "cancelled") { result.Status = ExecutionCancelled; return result, nil }
	if !validJSONObject(output) { return ExecutionResult{}, ErrInvalidExecutionResult }
	result.Status, result.Output = ExecutionSucceeded, RedactJSON(output)
	return result, nil
}

func n8nExecutionEndpoint(rawSnapshot json.RawMessage) (string, time.Duration, error) {
	var snapshot struct {
		WorkflowConnection struct {
			Type           string `json:"type"`
			BaseURL        string `json:"baseUrl"`
			TimeoutSeconds int    `json:"timeoutSeconds"`
		} `json:"workflowConnection"`
		WorkflowConfiguration struct {
			ResolvedWebhookPath string `json:"resolvedWebhookPath"`
			TypeConfig struct {
				ReferenceType  string `json:"referenceType"`
				ReferenceValue string `json:"referenceValue"`
			} `json:"typeConfig"`
		} `json:"workflowConfiguration"`
	}
	if json.Unmarshal(rawSnapshot, &snapshot) != nil ||
		snapshot.WorkflowConnection.Type != "n8n" ||
		snapshot.WorkflowConnection.TimeoutSeconds < 1 ||
		snapshot.WorkflowConnection.TimeoutSeconds > 300 {
		return "", 0, ErrExecutorUnavailable
	}
	reference := strings.TrimSpace(snapshot.WorkflowConfiguration.TypeConfig.ReferenceValue)
	if snapshot.WorkflowConfiguration.TypeConfig.ReferenceType == "workflow_id" {
		reference = strings.TrimSpace(snapshot.WorkflowConfiguration.ResolvedWebhookPath)
	} else if snapshot.WorkflowConfiguration.TypeConfig.ReferenceType != "webhook_path" {
		return "", 0, ErrExecutorUnavailable
	}
	if reference == "" || reference == "." || strings.HasPrefix(reference, "/") ||
		reference == ".." || strings.HasPrefix(reference, "../") ||
		path.Clean(reference) != reference || strings.ContainsAny(reference, "?#") {
		return "", 0, ErrExecutorUnavailable
	}
	endpoint, err := url.Parse(snapshot.WorkflowConnection.BaseURL)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil {
		return "", 0, ErrExecutorUnavailable
	}
	endpoint.Path = path.Join(endpoint.Path, "webhook", reference)
	endpoint.RawQuery, endpoint.Fragment = "", ""
	return endpoint.String(), time.Duration(snapshot.WorkflowConnection.TimeoutSeconds) * time.Second, nil
}

func n8nBaseEndpoint(rawSnapshot json.RawMessage) (string, time.Duration, error) {
	var snapshot struct {
		WorkflowConnection struct {
			Type           string `json:"type"`
			BaseURL        string `json:"baseUrl"`
			TimeoutSeconds int    `json:"timeoutSeconds"`
		} `json:"workflowConnection"`
	}
	if json.Unmarshal(rawSnapshot, &snapshot) != nil || snapshot.WorkflowConnection.Type != "n8n" || snapshot.WorkflowConnection.TimeoutSeconds < 1 || snapshot.WorkflowConnection.TimeoutSeconds > 300 {
		return "", 0, ErrExecutorUnavailable
	}
	endpoint, err := url.Parse(snapshot.WorkflowConnection.BaseURL)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil {
		return "", 0, ErrExecutorUnavailable
	}
	endpoint.RawQuery, endpoint.Fragment = "", ""
	return endpoint.String(), time.Duration(snapshot.WorkflowConnection.TimeoutSeconds) * time.Second, nil
}
