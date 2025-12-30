package postgres

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func CreateAssociation(ctx context.Context, db *gorm.DB, tenantID string, uaID, oaID uuid.UUID, ops []string) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		assoc := Association{
			TenantID:          tenantID,
			UserAttributeID:   uaID,
			ObjectAttributeID: oaID,
		}
		// Upsert association
		if err := tx.
			Where("tenant_id = ? AND user_attribute_id = ? AND object_attribute_id = ?", tenantID, uaID, oaID).
			FirstOrCreate(&assoc).Error; err != nil {
			return err
		}

		for _, op := range ops {
			row := AssociationOperation{
				TenantID:      tenantID,
				AssociationID: assoc.ID,
				Operation:     op,
			}
			// Upsert op
			if err := tx.
				Where("tenant_id = ? AND association_id = ? AND operation = ?", tenantID, assoc.ID, op).
				FirstOrCreate(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
