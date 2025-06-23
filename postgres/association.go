package postgres

import (
	"github.com/kumarabd/policy-machine/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *postgres) AddAssociation(r *model.Association) error {
	return h.AddAssociationBulk([]*model.Association{r})
}

func (h *postgres) AddAssociationBulk(Associations []*model.Association) error {
	processedRelationships := make(map[string]struct{})
	return h.handler.Transaction(func(tx *gorm.DB) error {
		for _, r := range Associations {
			if _, ok := processedRelationships[r.RelationshipID]; !ok {
				if err := tx.
					Session(&gorm.Session{FullSaveAssociations: true}).
					Where("relationship_id = ? AND class_name = ?", r.RelationshipID, r.ClassName).
					FirstOrCreate(r).Error; err != nil {
					return err
				}
				processedRelationships[r.RelationshipID] = struct{}{}
			}
		}
		return nil
	})
}

func (h *postgres) AddActionsToAssociations(association *model.Association, verbs []string) error {
	return h.handler.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("relationship_id = ? AND class_name = ?", association.RelationshipID, association.ClassName).First(association).Error; err != nil {
			return err
		}

		association.AddVerbs(verbs)

		return tx.Save(&association).Error
	})
}

func (h *postgres) FetchAssociation(ass *model.Association, preload bool) error {
	exec := h.handler.Where("relationship_id = ? AND class_name = ?", ass.RelationshipID, ass.ClassName)
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.First(ass)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (h *postgres) FetchAssociationBulk(preload bool) ([]*model.Association, error) {
	var associations []*model.Association
	exec := h.handler
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.Find(&associations)
	if result.Error != nil {
		return nil, result.Error
	}
	return associations, nil
}
