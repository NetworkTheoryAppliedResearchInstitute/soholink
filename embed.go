package soholink

import "embed"

//go:embed configs/default.yaml
var DefaultConfigYAML []byte

//go:embed configs/policies/default.rego
var DefaultPolicyRego []byte

// PoliciesFS holds all .rego files under configs/policies/ at compile time.
// The binary uses this so no external configs/ directory is required at runtime.
//
//go:embed configs/policies
var PoliciesFS embed.FS

// DashboardFS is intentionally removed. The web dashboard has been replaced
// by a FlutterFlow-generated native application that connects to the REST API.
