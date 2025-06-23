package model

import (
	"fmt"

	"github.com/kumarabd/policy-machine/utils"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

const (
	EntityKey EntityString = "entity"
)

type EntityString string

func (e EntityString) String() string {
	return string(e)
}

type Entity struct {
	gorm.Model
	HashID      string       `gorm:"primaryKey;uniqueIndex"`
	Type        EntityString `gorm:"index"`
	Name        string
	Obligations pq.StringArray `gorm:"type:text[]"`
	Conditions  pq.StringArray `gorm:"type:text[]"`
}

func (n *Entity) GetName() string {
	return n.Name
}

func (n *Entity) GetType() EntityString {
	return n.Type
}

func (n *Entity) GetID() string {
	return n.HashID
}

func (n *Entity) Init(name string, entityType EntityString, props map[string]string) {
	// Create a new map with props values
	newProps := make(map[string]string)
	for k, v := range props {
		newProps[k] = v
	}
	// Generate id hash
	newProps[NameAttribute.String()] = name
	newProps[EntityKey.String()] = entityType.String()
	hash, err := utils.GenerateJWTID(newProps, "secretKey")
	if err != nil {
		fmt.Println("unable to generate id", err)
		return
	}

	n.HashID = hash
	n.Name = name
	n.Type = entityType
}
