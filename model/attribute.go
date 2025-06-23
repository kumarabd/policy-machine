package model

import (
	"gorm.io/gorm"
)

const (
	ResourceAttributeEntity EntityString = "resource_attribute"
	SubjectAttributeEntity  EntityString = "subject_attribute"

	NameAttribute AttributeString = "name"
)

type AttributeString string

func (a AttributeString) String() string {
	return string(a)
}

type Attribute struct {
	gorm.Model
	EntityID     string         `gorm:"primaryKey;uniqueIndex"`
	Entity       *Entity        `gorm:"foreignKey:HashID;references:EntityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Assignments  []Relationship `gorm:"foreignKey:FromID;references:EntityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Associations []Relationship `gorm:"foreignKey:FromID;references:EntityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	
	// NGAC Prohibitions - relationships that deny access  
	Prohibitions []Relationship `gorm:"foreignKey:FromID;references:EntityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	
	Properties   []*Property    `gorm:"many2many:attribute_properties;"`
}

func (a *Attribute) DeepCopy() Entity {
	entity := *a.Entity
	return entity
}

func (a *Attribute) Init(id string, entity EntityString, props map[string]string) {
	a.Entity = &Entity{}
	a.Entity.Init(id, entity, props)
	a.EntityID = a.Entity.HashID
	a.Assignments = make([]Relationship, 0)
	a.Associations = make([]Relationship, 0)
	a.Prohibitions = make([]Relationship, 0)
	a.Properties = MapToProperty(props)
}

func (a *Attribute) AddAssignments(assignments []Relationship) {
	temp := make(map[string]struct{})
	for _, ass := range a.Assignments {
		temp[ass.HashID] = struct{}{}
	}
	for _, ass := range assignments {
		if _, exists := temp[ass.HashID]; !exists {
			a.Assignments = append(a.Assignments, ass)
		}
	}
}

func (a *Attribute) AddAssociations(associations []Relationship) {
	a.Associations = append(a.Associations, associations...)
}

// AddProhibitions adds prohibition relationships to this attribute (for user attributes)
func (a *Attribute) AddProhibitions(prohibitions []Relationship) {
	temp := make(map[string]struct{})
	for _, proh := range a.Prohibitions {
		temp[proh.HashID] = struct{}{}
	}
	for _, proh := range prohibitions {
		if proh.Type == ProhibitionRelationship {
			if _, exists := temp[proh.HashID]; !exists {
				a.Prohibitions = append(a.Prohibitions, proh)
			}
		}
	}
}

// GetProhibitionRelationships returns all prohibition relationships from this attribute
func (a *Attribute) GetProhibitionRelationships() []Relationship {
	var result []Relationship
	for _, relationship := range a.Prohibitions {
		if relationship.Type == ProhibitionRelationship {
			result = append(result, relationship)
		}
	}
	return result
}

// HasProhibitionTo checks if this attribute has a prohibition relationship to the target
func (a *Attribute) HasProhibitionTo(targetID string) bool {
	for _, prohibition := range a.Prohibitions {
		if prohibition.Type == ProhibitionRelationship && prohibition.ToID == targetID {
			return true
		}
	}
	return false
}
