package moderation

import (
	"fmt"
	"strings"
)

// AllowedPurposeCategories lists valid workload purpose declarations.
var AllowedPurposeCategories = map[string]bool{
	"data_processing":    true,
	"ml_training":        true,
	"rendering":          true,
	"web_serving":        true,
	"simulation":         true,
	"scientific_compute": true,
	"media_encoding":     true,
	"security_monitoring": true,
	"conferencing":       true,
	"other":              true,
}

// hardwareAllowedCategories lists purpose categories that may declare hardware access.
var hardwareAllowedCategories = map[string]bool{
	"security_monitoring": true,
	"scientific_compute":  true,
	"other":               true,
}

// WorkloadManifest is a required declaration of intent for every workload
// purchase. It is validated at submission time and evaluated by OPA safety
// policies before the workload is scheduled.
//
// Requesters certify the accuracy of all fields. False declarations may result
// in DID suspension and law-enforcement referral per the platform Terms of Service.
type WorkloadManifest struct {
	// Required
	PurposeCategory string `json:"purpose_category"` // must be in AllowedPurposeCategories

	// Required — min 20 characters
	Description string `json:"description"`

	// Required — one of "none" | "declared_only" | "unrestricted"
	NetworkAccess string `json:"network_access"`

	// Required when NetworkAccess != "none"
	ExternalEndpoints []string `json:"external_endpoints"`

	// Optional
	HardwareAccess     bool     `json:"hardware_access"`      // GPIO/serial/USB
	Capabilities       []string `json:"capabilities"`          // e.g. ["cpu","gpu","capture_video"]
	OutputDestinations []string `json:"output_destinations"`
	DataSources        []string `json:"data_sources"`
	WasmCID            string   `json:"wasm_cid,omitempty"`
}

// ValidateManifest returns a slice of validation error strings.
// An empty slice means the manifest is valid.
func ValidateManifest(m *WorkloadManifest) []string {
	var errs []string

	// purpose_category
	if m.PurposeCategory == "" {
		errs = append(errs, "purpose_category is required")
	} else if !AllowedPurposeCategories[m.PurposeCategory] {
		keys := make([]string, 0, len(AllowedPurposeCategories))
		for k := range AllowedPurposeCategories {
			keys = append(keys, k)
		}
		errs = append(errs, fmt.Sprintf("purpose_category must be one of: %s", strings.Join(keys, ", ")))
	}

	// description
	if len(strings.TrimSpace(m.Description)) < 20 {
		errs = append(errs, "description must be at least 20 characters")
	}

	// network_access
	switch m.NetworkAccess {
	case "none", "declared_only", "unrestricted":
		// valid
	case "":
		errs = append(errs, "network_access is required (none|declared_only|unrestricted)")
	default:
		errs = append(errs, fmt.Sprintf("network_access must be none, declared_only, or unrestricted; got %q", m.NetworkAccess))
	}

	// external_endpoints required when network_access != "none"
	if m.NetworkAccess == "declared_only" && len(m.ExternalEndpoints) == 0 {
		errs = append(errs, "external_endpoints must list at least one endpoint when network_access is declared_only")
	}

	// hardware_access requires a recognized purpose category
	if m.HardwareAccess && !hardwareAllowedCategories[m.PurposeCategory] {
		errs = append(errs, fmt.Sprintf(
			"hardware_access=true requires purpose_category to be one of: security_monitoring, scientific_compute, other; got %q",
			m.PurposeCategory,
		))
	}

	return errs
}
