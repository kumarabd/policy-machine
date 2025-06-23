package postgres

import (
	"github.com/kumarabd/policy-machine/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *postgres) AddProhibition(r *model.Prohibition) error {
	return h.AddProhibitionBulk([]*model.Prohibition{r})
}

func (h *postgres) AddProhibitionBulk(Prohibitions []*model.Prohibition) error {
	processedRelationships := make(map[string]struct{})
	return h.handler.Transaction(func(tx *gorm.DB) error {
		for _, r := range Prohibitions {
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

func (h *postgres) AddActionsToProhibitions(prohibition *model.Prohibition, verbs []string) error {
	return h.handler.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("relationship_id = ? AND class_name = ?", prohibition.RelationshipID, prohibition.ClassName).First(prohibition).Error; err != nil {
			return err
		}

		prohibition.AddVerbs(verbs)

		return tx.Save(&prohibition).Error
	})
}

func (h *postgres) FetchProhibition(ass *model.Prohibition, preload bool) error {
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

func (h *postgres) FetchProhibitionBulk(preload bool) ([]*model.Prohibition, error) {
	var prohibitions []*model.Prohibition
	exec := h.handler
	if preload {
		exec = exec.Preload(clause.Associations)
	}

	result := exec.Find(&prohibitions)
	if result.Error != nil {
		return nil, result.Error
	}
	return prohibitions, nil
}

func (h *postgres) DeleteProhibition(prohibition *model.Prohibition) error {
	return h.handler.Transaction(func(tx *gorm.DB) error {
		return tx.
			Where("relationship_id = ? AND class_name = ?", prohibition.RelationshipID, prohibition.ClassName).
			Delete(prohibition).
			Error
	})
}

func (h *postgres) FetchProhibitionsForPolicyClass(className string) ([]*model.Prohibition, error) {
	var prohibitions []*model.Prohibition
	result := h.handler.Where("class_name = ?", className).Find(&prohibitions)
	if result.Error != nil {
		return nil, result.Error
	}
	return prohibitions, nil
}
