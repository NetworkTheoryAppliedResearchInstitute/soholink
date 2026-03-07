// Package audit contains cross-cutting tests that verify architectural
// invariants which no single unit test can enforce — specifically the "wiring"
// requirements that main() must satisfy before app.New() is called.
//
// These tests run without any build tag so they execute in every CI job,
// including the headless (non-GUI) matrix legs.
package audit_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// requiredMainCalls lists symbols that cmd/soholink/main.go must call before
// app.New(). Add a line here whenever you introduce a new package-level
// registration function so this test catches any entry point that forgets it.
//
// Format: the exact byte sequence that must appear somewhere in main.go.
var requiredMainCalls = []string{
	"config.SetDefaultConfig",
	"cli.SetDefaultPolicy",
	// policy.SetEmbeddedFS is intentionally absent: the embedded FS is now
	// wired inside app.New() via an explicit argument to policy.NewEngine,
	// so no call in main.go is needed or wanted.
}

// binaryEntryPoints lists all cmd/*/main.go files that must contain every
// symbol in requiredMainCalls.  Add a new entry when a new binary is added
// under cmd/.
var binaryEntryPoints = []string{
	filepath.Join("cmd", "soholink", "main.go"),  // GUI binary (//go:build gui)
	filepath.Join("cmd", "fedaaa", "main.go"),    // headless CLI binary
}

// TestMainWiresAllRegistrations reads every binary entry point as text and
// asserts that each symbol in requiredMainCalls appears at least once.
//
// Why read source rather than import the package?  cmd/soholink/main.go
// carries a //go:build gui constraint that requires CGO + Fyne.  A text scan
// lets this test run everywhere without those build dependencies.
func TestMainWiresAllRegistrations(t *testing.T) {
	for _, relPath := range binaryEntryPoints {
		mainPath := resolveFromRoot(t, relPath)

		src, err := os.ReadFile(mainPath)
		if err != nil {
			t.Fatalf("could not read %s: %v", mainPath, err)
		}

		for _, sym := range requiredMainCalls {
			if !bytes.Contains(src, []byte(sym)) {
				t.Errorf("%s is missing required wiring call: %s\n"+
					"  → Add %s() before the first app.New() call and document it\n"+
					"    in the Startup wiring comment block at the top of main().", relPath, sym, sym)
			}
		}
	}
}

// TestMainHasWiringComment verifies that the startup wiring comment block
// exists in every binary entry point.  A future developer adding a new
// registration function is more likely to notice (and update) an explicit
// checklist than to scan bare call sites.
func TestMainHasWiringComment(t *testing.T) {
	marker := []byte("Startup wiring")
	for _, relPath := range binaryEntryPoints {
		mainPath := resolveFromRoot(t, relPath)

		src, err := os.ReadFile(mainPath)
		if err != nil {
			t.Fatalf("could not read %s: %v", mainPath, err)
		}

		if !bytes.Contains(src, marker) {
			t.Errorf("%s is missing the \"Startup wiring\" comment block.\n"+
				"  This comment documents which package-level registration calls must\n"+
				"  appear before app.New().  Add it back so future contributors know\n"+
				"  to update the list when they add a new registration function.", relPath)
		}
	}
}

// TestMainParsesCleanly ensures every binary entry point is syntactically
// valid Go, even when build tags are absent (parser ignores them).
// A syntax error prevents deadcode and other tooling from analysing the file.
func TestMainParsesCleanly(t *testing.T) {
	fset := token.NewFileSet()
	for _, relPath := range binaryEntryPoints {
		mainPath := resolveFromRoot(t, relPath)
		// parser.ParseFile ignores build constraints; it checks syntax only.
		if _, err := parser.ParseFile(fset, mainPath, nil, parser.AllErrors); err != nil {
			t.Errorf("%s has syntax errors: %v", relPath, err)
		}
	}
}

// resolveFromRoot finds the repository root by walking up from this test file's
// location (internal/audit/) and then appends the given relative path.
// parts may be a single pre-joined path or multiple segments.
func resolveFromRoot(t *testing.T, parts ...string) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot determine repository root")
	}

	// Walk up: internal/audit/wiring_test.go → internal/audit → internal → repo root
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	return filepath.Join(append([]string{root}, parts...)...)
}
