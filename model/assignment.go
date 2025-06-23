package model

import "gorm.io/gorm"

const (
	AssignmentRelationship RelationshipString = "assignment"
)

type Assignment struct {
	gorm.Model
	RelationshipID string        `gorm:"primaryKey;uniqueIndex"`
	Relationship   *Relationship `gorm:"foreignKey:HashID;references:RelationshipID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (a *Assignment) DeepCopy() Relationship {
	relationship := *a.Relationship
	return relationship
}

func (a *Assignment) Init(from Entity, to Entity) {
	a.Relationship = &Relationship{}
	a.Relationship.Init(from, to, AssignmentRelationship)
	a.RelationshipID = a.Relationship.HashID
}
