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
)

const maxWorkflowResponseBytes = 2 << 20

type N8NWorkflowExecutor struct {
	client *http.Client
}

func NewN8NWorkflowExecutor(client *http.Client) *N8NWorkflowExecutor {
	if client == nil {
		client = &http.Client{}
	}
	return &N8NWorkflowExecutor{client: client}
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
		return ExecutionResult{Status: ExecutionFailed, ErrorCode: "upstream_http_error", ErrorMessage: "workflow execution failed"}, nil
	}
	output, err := io.ReadAll(io.LimitReader(response.Body, maxWorkflowResponseBytes+1))
	if err != nil || len(output) > maxWorkflowResponseBytes || !validJSONObject(output) {
		return ExecutionResult{Status: ExecutionFailed, ErrorCode: "invalid_response", ErrorMessage: "workflow execution failed"}, nil
	}
	return ExecutionResult{
		Status: ExecutionSucceeded, Output: json.RawMessage(output),
		ExternalExecutionID: strings.TrimSpace(response.Header.Get("X-N8N-Execution-Id")),
		Metadata:            map[string]string{},
	}, nil
}

func (e *N8NWorkflowExecutor) Cancel(ctx context.Context, request ExecutionRequest) (ExecutionResult, error) {
	return e.executionControl(ctx, request, http.MethodPost)
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
	if method == http.MethodPost {
		endpoint.Path = path.Join(endpoint.Path, "stop")
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestCtx, method, endpoint.String(), nil)
	if err != nil {
		return ExecutionResult{}, err
	}
	response, err := e.client.Do(httpRequest)
	if err != nil {
		if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return ExecutionResult{}, ErrExecutionTimeout
		}
		return ExecutionResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusConflict {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return ExecutionResult{Status: ExecutionCancelled, ExternalExecutionID: request.ExternalExecutionID}, nil
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return ExecutionResult{Status: ExecutionFailed, ErrorCode: "upstream_http_error", ErrorMessage: "workflow execution failed"}, nil
	}
	if method == http.MethodPost {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return ExecutionResult{Status: ExecutionCancelled, ExternalExecutionID: request.ExternalExecutionID}, nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxWorkflowResponseBytes+1))
	if err != nil || len(body) > maxWorkflowResponseBytes {
		return ExecutionResult{}, ErrInvalidExecutionResult
	}
	return parseN8NQueryResponse(body, request.ExternalExecutionID)
}

// parseN8NQueryResponse accepts only the small n8n execution representation
// required for restart recovery. It intentionally does not try to normalize
// arbitrary n8n API versions or preserve an upstream response body.
func parseN8NQueryResponse(body json.RawMessage, externalExecutionID string) (ExecutionResult, error) {
	var response struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
		Output json.RawMessage `json:"output"`
	}
	if !validJSONObject(body) || json.Unmarshal(body, &response) != nil {
		return ExecutionResult{}, ErrInvalidExecutionResult
	}
	result := ExecutionResult{ExternalExecutionID: externalExecutionID}
	switch strings.ToLower(strings.TrimSpace(response.Status)) {
	case "new", "waiting", "running":
		result.Status = ExecutionRunning
	case "success", "succeeded":
		output := response.Output
		if len(output) == 0 {
			output = response.Data
		}
		if !validJSONObject(output) {
			return ExecutionResult{}, ErrInvalidExecutionResult
		}
		result.Status, result.Output = ExecutionSucceeded, RedactJSON(output)
	case "error", "failed", "crashed":
		result.Status, result.ErrorCode, result.ErrorMessage = ExecutionFailed, "upstream_execution_failed", "workflow execution failed"
	case "canceled", "cancelled":
		result.Status = ExecutionCancelled
	default:
		return ExecutionResult{}, ErrInvalidExecutionResult
	}
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
			TypeConfig struct {
				ReferenceType  string `json:"referenceType"`
				ReferenceValue string `json:"referenceValue"`
			} `json:"typeConfig"`
		} `json:"workflowConfiguration"`
	}
	if json.Unmarshal(rawSnapshot, &snapshot) != nil ||
		snapshot.WorkflowConnection.Type != "n8n" ||
		snapshot.WorkflowConfiguration.TypeConfig.ReferenceType != "webhook_path" ||
		snapshot.WorkflowConnection.TimeoutSeconds < 1 ||
		snapshot.WorkflowConnection.TimeoutSeconds > 300 {
		return "", 0, ErrExecutorUnavailable
	}
	reference := strings.TrimSpace(snapshot.WorkflowConfiguration.TypeConfig.ReferenceValue)
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
