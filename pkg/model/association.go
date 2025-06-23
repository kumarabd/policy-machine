package model

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
)

const (
	AssociationRelationship RelationshipString = "association"
)

type Association struct {
	gorm.Model
	RelationshipID string         `gorm:"primaryKey;uniqueIndex"`
	Relationship   *Relationship  `gorm:"foreignKey:HashID;references:RelationshipID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Verbs          pq.StringArray `gorm:"type:text[]"`
	ClassName      string         `gorm:"index"`
	Obligations    pq.StringArray `gorm:"type:text[]"`
	Conditions     pq.StringArray `gorm:"type:text[]"`
}

func (a *Association) DeepCopy() Relationship {
	relationship := *a.Relationship
	return relationship
}

func (a *Association) AddVerbs(verbs []string) {
	temp := make(map[string]struct{})
	for _, verb := range a.Verbs {
		temp[verb] = struct{}{}
	}
	for _, verb := range verbs {
		if _, exists := temp[verb]; !exists {
			a.Verbs = append(a.Verbs, verb)
		}
	}
}

func (a *Association) Init(from Entity, to Entity, verbs []string, className string) {
	a.Relationship = &Relationship{}
	a.Relationship.Init(from, to, AssociationRelationship)
	a.RelationshipID = a.Relationship.HashID
	a.ClassName = className
	a.Verbs = verbs
}
