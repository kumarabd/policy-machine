package model

import (
	"gorm.io/gorm"
)

const (
	SubjectEntity EntityString = "subject"

	Agent SubjectString = "agent"
)

type SubjectString string

func (a SubjectString) String() string {
	return string(a)
}

type Subject struct {
	gorm.Model
	EntityID    string         `gorm:"primaryKey;uniqueIndex"`
	Entity      *Entity        `gorm:"foreignKey:HashID;references:EntityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Assignments []Relationship `gorm:"foreignKey:FromID;references:EntityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`

	// NGAC Prohibitions - direct prohibition relationships on this subject
	Prohibitions []Relationship `gorm:"foreignKey:FromID;references:EntityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`

	Properties []*Property `gorm:"many2many:subject_properties;joinForeignKey:subject_id;joinReferences:property_id"`
}

func (s *Subject) DeepCopy() Entity {
	entity := *s.Entity
	return entity
}

func (s *Subject) Init(id string, props map[string]string) {
	s.Entity = &Entity{}
	s.Entity.Init(id, SubjectEntity, props)
	s.EntityID = s.Entity.HashID
	s.Assignments = make([]Relationship, 0)
	s.Prohibitions = make([]Relationship, 0)
	s.Properties = MapToProperty(props)
}

func (s *Subject) AddAssignments(assignments []Relationship) {
	temp := make(map[string]struct{})
	for _, ass := range s.Assignments {
		temp[ass.HashID] = struct{}{}
	}
	for _, ass := range assignments {
		if _, exists := temp[ass.HashID]; !exists {
			s.Assignments = append(s.Assignments, ass)
		}
	}
}

// AddProhibitions adds prohibition relationships to this subject
func (s *Subject) AddProhibitions(prohibitions []Relationship) {
	temp := make(map[string]struct{})
	for _, proh := range s.Prohibitions {
		temp[proh.HashID] = struct{}{}
	}
	for _, proh := range prohibitions {
		if proh.Type == ProhibitionRelationship {
			if _, exists := temp[proh.HashID]; !exists {
				s.Prohibitions = append(s.Prohibitions, proh)
			}
		}
	}
}

// GetProhibitionRelationships returns all prohibition relationships from this subject
func (s *Subject) GetProhibitionRelationships() []Relationship {
	var result []Relationship
	for _, relationship := range s.Prohibitions {
		if relationship.Type == ProhibitionRelationship {
			result = append(result, relationship)
		}
	}
	return result
}

// HasProhibitionTo checks if this subject has a prohibition relationship to the target
func (s *Subject) HasProhibitionTo(targetID string) bool {
	for _, prohibition := range s.Prohibitions {
		if prohibition.Type == ProhibitionRelationship && prohibition.ToID == targetID {
			return true
		}
	}
	return false
}
