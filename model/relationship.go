package model

import (
	"fmt"

	"github.com/kumarabd/policy-machine/utils"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

const (
	RelationshipKey RelationshipString = "relationship"
)

type RelationshipString string

func (e RelationshipString) String() string {
	return string(e)
}

type Relationship struct {
	gorm.Model
	HashID      string             `gorm:"primaryKey;uniqueIndex"`
	FromID      string             `gorm:"index"`
	ToID        string             `gorm:"index"`
	Type        RelationshipString `gorm:"index"`
	Obligations pq.StringArray     `gorm:"type:text[]"`
	Conditions  pq.StringArray     `gorm:"type:text[]"`
}

func (n *Relationship) GetID() string {
	return n.HashID
}

func (n *Relationship) Init(from Entity, to Entity, rtype RelationshipString) {
	// Create a new map with props values
	props := map[string]string{
		RelationshipKey.String(): rtype.String(),
		"from":                   from.GetID(),
		"to":                     to.GetID(),
	}
	hash, err := utils.GenerateJWTID(props, "secretKey")
	if err != nil {
		fmt.Println("unable to generate id", err)
		return
	}

	n.HashID = hash
	n.FromID = from.GetID()
	n.ToID = to.GetID()
	n.Type = rtype
}

func (n *Relationship) GetType() RelationshipString {
	return n.Type
}
