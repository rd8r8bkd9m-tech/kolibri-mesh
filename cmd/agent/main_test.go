package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecAuthorizationRequiresTokenByDefault(t *testing.T) {
	t.Setenv("KOLIBRI_MESH_EXEC_TOKEN", "")
	t.Setenv("MESH_EXEC_TOKEN", "")
	t.Setenv("KOLIBRI_MESH_EXEC_ALLOW_INSECURE", "")

	req := httptest.NewRequest(http.MethodPost, "/api/exec", nil)
	if execAuthorized(req) {
		t.Fatal("exec should not be authorized without a token or explicit insecure override")
	}

	t.Setenv("KOLIBRI_MESH_EXEC_ALLOW_INSECURE", "1")
	if !execAuthorized(req) {
		t.Fatal("exec should allow insecure mode only when explicitly enabled")
	}
}

func TestExecAuthorizationAcceptsBearerOrMeshHeader(t *testing.T) {
	t.Setenv("KOLIBRI_MESH_EXEC_TOKEN", "test-token")
	t.Setenv("MESH_EXEC_TOKEN", "")
	t.Setenv("KOLIBRI_MESH_EXEC_ALLOW_INSECURE", "")

	bearerReq := httptest.NewRequest(http.MethodPost, "/api/exec", nil)
	bearerReq.Header.Set("Authorization", "Bearer test-token")
	if !execAuthorized(bearerReq) {
		t.Fatal("bearer token should authorize exec")
	}

	headerReq := httptest.NewRequest(http.MethodPost, "/api/exec", nil)
	headerReq.Header.Set("X-Kolibri-Mesh-Token", "test-token")
	if !execAuthorized(headerReq) {
		t.Fatal("mesh token header should authorize exec")
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/exec", nil)
	badReq.Header.Set("Authorization", "Bearer wrong-token")
	if execAuthorized(badReq) {
		t.Fatal("wrong token must not authorize exec")
	}
}

func TestHandleExecRejectsUnauthorizedAndAudits(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "exec-audit.jsonl")
	t.Setenv("KOLIBRI_MESH_EXEC_TOKEN", "test-token")
	t.Setenv("KOLIBRI_MESH_EXEC_AUDIT_LOG", auditPath)

	req := httptest.NewRequest(http.MethodPost, "/api/exec", strings.NewReader(`{"command":"echo","args":["hello"]}`))
	rec := httptest.NewRecorder()

	handleExec(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized response, got %d", rec.Code)
	}
	auditBytes, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if !strings.Contains(string(auditBytes), `"authorized":false`) || !strings.Contains(string(auditBytes), "unauthorized") {
		t.Fatalf("audit log does not record unauthorized request: %s", string(auditBytes))
	}
}

func TestHandleExecRunsWithTokenAndRedactsAudit(t *testing.T) {
	auditPath := filepath.Join(t.TempDir(), "exec-audit.jsonl")
	t.Setenv("KOLIBRI_MESH_EXEC_TOKEN", "test-token")
	t.Setenv("KOLIBRI_MESH_EXEC_AUDIT_LOG", auditPath)

	reqBody := `{"command":"sh","args":["-c","printf done","--password","clear-text"],"timeout":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/exec", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	handleExec(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected ok response, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ExecResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Stdout != "done" || resp.ExitCode != 0 || resp.Error != "" {
		t.Fatalf("unexpected exec response: %+v", resp)
	}

	auditBytes, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	audit := string(auditBytes)
	if !strings.Contains(audit, "[redacted]") {
		t.Fatalf("audit log should redact sensitive argument: %s", audit)
	}
	if strings.Contains(audit, "clear-text") {
		t.Fatalf("audit log leaked sensitive argument: %s", audit)
	}
}
