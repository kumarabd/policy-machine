package model

import (
	"gorm.io/gorm"
)

const (
	ResourceEntity EntityString = "resource"

	Server ResourceString = "server"
)

type ResourceString string

func (a ResourceString) String() string {
	return string(a)
}

type Resource struct {
	gorm.Model
	EntityID    string         `gorm:"primaryKey;uniqueIndex"`
	Entity      *Entity        `gorm:"foreignKey:HashID;references:EntityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Assignments []Relationship `gorm:"foreignKey:FromID;references:EntityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Properties  []*Property    `gorm:"many2many:resource_properties;"`
}

func (r *Resource) DeepCopy() Entity {
	entity := *r.Entity
	return entity
}

func (r *Resource) Init(id string, props map[string]string) {
	r.Entity = &Entity{}
	r.Entity.Init(id, ResourceEntity, props)
	r.EntityID = r.Entity.HashID
	r.Assignments = make([]Relationship, 0)
	r.Properties = MapToProperty(props)
}

func (r *Resource) AddAssignments(assignments []Relationship) {
	temp := make(map[string]struct{})
	for _, ass := range r.Assignments {
		temp[ass.HashID] = struct{}{}
	}
	for _, ass := range assignments {
		if _, exists := temp[ass.HashID]; !exists {
			r.Assignments = append(r.Assignments, ass)
		}
	}
}
