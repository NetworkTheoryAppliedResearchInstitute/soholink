package policy_test

// embedded_test.go verifies that policy.NewEngine correctly falls back to a
// compiled-in fs.FS when policyDir is empty.  It imports the root soholink
// package (which holds the embedded .rego files) and exercises the same code
// path that app.New() uses at runtime.
//
// If this test fails it means one of three things went wrong:
//   1. The .rego files were removed from configs/policies/ (embed directive broken)
//   2. The fs.Sub prefix ("configs/policies") no longer matches the embed path
//   3. NewEngine's fallback branch has a bug
//
// Any wiring gap between the caller and NewEngine is caught here before it
// reaches a running binary.

import (
	"io/fs"
	"strings"
	"testing"

	soholink "github.com/NetworkTheoryAppliedResearchInstitute/soholink"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/policy"
)

// TestNewEngineWithEmbeddedPolicies is the primary startup integration test.
// It mirrors exactly what app.New() does at runtime and must pass in every CI
// configuration (no GUI tag, no external files required).
func TestNewEngineWithEmbeddedPolicies(t *testing.T) {
	sub, err := fs.Sub(soholink.PoliciesFS, "configs/policies")
	if err != nil {
		t.Fatalf("fs.Sub(PoliciesFS, \"configs/policies\") failed: %v", err)
	}

	engine, err := policy.NewEngine("", sub)
	if err != nil {
		t.Fatalf("NewEngine with embedded FS failed: %v\n"+
			"This likely means the embedded .rego files are missing or the fs.Sub prefix is wrong.", err)
	}

	if dir := engine.PolicyDir(); dir != "(embedded)" {
		t.Errorf("PolicyDir = %q, want \"(embedded)\"", dir)
	}

	files, err := engine.PolicyFiles()
	if err != nil {
		t.Fatalf("PolicyFiles() error: %v", err)
	}
	if len(files) == 0 {
		t.Error("PolicyFiles() returned empty list; at least one .rego file must be embedded")
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".rego") {
			t.Errorf("unexpected non-.rego file in embedded policy set: %s", f)
		}
	}
}

// TestNewEngineFailsWithNoSource confirms the error message when the caller
// supplies neither a policyDir nor a fallback — this is the scenario that
// previously caused a cryptic runtime panic deep in app.New().
func TestNewEngineFailsWithNoSource(t *testing.T) {
	_, err := policy.NewEngine("", nil)
	if err == nil {
		t.Fatal("NewEngine(\"\", nil) should return an error; got nil")
	}
	// The error message should be actionable.
	if !strings.Contains(err.Error(), "policyDir is empty") &&
		!strings.Contains(err.Error(), "no policy source") {
		t.Errorf("unexpected error text %q; expected it to mention policyDir or no policy source", err)
	}
}
