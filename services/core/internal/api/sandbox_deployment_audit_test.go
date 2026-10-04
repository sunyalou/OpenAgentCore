package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MiniMax-AI/OpenAgentCore/services/core/internal/adminaudit"
	"github.com/MiniMax-AI/OpenAgentCore/services/core/internal/deployment"
	"github.com/MiniMax-AI/OpenAgentCore/services/core/internal/sandbox"
	"github.com/MiniMax-AI/OpenAgentCore/services/core/internal/sandbox/providers"
)

// A committed deployment change records an administrator mutation, so the
// handler must carry an administrator audit source into the operation. Without
// it the committing transaction rejects the record and the change fails with
// 400 invalid_request, while an identical no-op selection still succeeds.
func TestSandboxDeploymentMutationsCarryAdministratorAuditSource(t *testing.T) {
	deps, fakes := sandboxFakes(t)
	audited := func(name string) func(context.Context, sandbox.Selection) (deployment.View, error) {
		return func(ctx context.Context, in sandbox.Selection) (deployment.View, error) {
			source, ok := adminaudit.FromContext(ctx)
			if !ok || source.ProjectID != "" || source.CredentialID == "" {
				t.Errorf("%s carried no deployment audit source: %+v", name, source)
			}
			if err := source.ValidateDeploymentMutation("change", "sandbox_deployment", "installation"); err != nil {
				t.Errorf("%s audit source cannot be recorded: %+v", name, source)
			}
			return deployment.View{Provider: in.Provider}, nil
		}
	}
	fakes.deploymentChanges.initializeSandboxDeployment = audited("initialize")
	fakes.deploymentChanges.updateSandboxDeployment = audited("change")
	fakes.deployment.decodeConfiguration = providers.Builtin().DecodeInput
	h := newTestHandler(t, deps)
	const selection = `{"provider":"e2b","expected_generation":2,"credential":{"api_key":"synthetic-private-key"},"configuration":{"template":"qualified:build"}}`
	for _, test := range []struct{ method, body string }{
		{http.MethodPost, strings.Replace(selection, `"expected_generation":2`, `"expected_generation":0`, 1)},
		{http.MethodPut, selection},
	} {
		request := httptest.NewRequest(test.method, "/core/v1/sandbox/deployment", strings.NewReader(test.body))
		request.Header.Set("Authorization", "Bearer administrator")
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s %s", test.method, response.Body)
		}
	}
}
