package central

import (
	"context"
	"fmt"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// TenantManager handles multi-tenant lifecycle for central SOHO.
type TenantManager struct {
	store *store.Store
}

// NewTenantManager creates a new tenant manager.
func NewTenantManager(s *store.Store) *TenantManager {
	return &TenantManager{store: s}
}

// RegisterTenant creates a new thin-client tenant record.
func (m *TenantManager) RegisterTenant(ctx context.Context, tenantDID string, name string) error {
	return m.store.CreateTenant(ctx, &store.TenantRow{
		TenantID:  tenantDID,
		Name:      name,
		Status:    "active",
		CreatedAt: time.Now(),
	})
}

// SuspendTenant marks a tenant as suspended.
func (m *TenantManager) SuspendTenant(ctx context.Context, tenantDID string, reason string) error {
	return m.store.UpdateTenantStatus(ctx, tenantDID, "suspended")
}

// GetTenant retrieves a tenant record.
func (m *TenantManager) GetTenant(ctx context.Context, tenantDID string) (*store.TenantRow, error) {
	return m.store.GetTenant(ctx, tenantDID)
}

// ListTenants lists all tenants.
func (m *TenantManager) ListTenants(ctx context.Context) ([]store.TenantRow, error) {
	return m.store.ListTenants(ctx)
}

// RecordRevenue records a revenue split for a completed transaction.
func (m *TenantManager) RecordRevenue(ctx context.Context, fee TransactionFee, tenantID string) error {
	row := &store.CentralRevenueRow{
		RevenueID:      fmt.Sprintf("rev_%d", time.Now().UnixNano()),
		TransactionID:  fee.TransactionID,
		TenantID:       tenantID,
		TotalAmount:    fee.TotalAmount,
		CentralFee:     fee.CentralFee,
		ProducerPayout: fee.ProducerPayout,
		ProcessorFee:   fee.ProcessorFee,
		Currency:       fee.Currency,
		CreatedAt:      fee.CreatedAt,
	}
	return m.store.CreateCentralRevenue(ctx, row)
}
