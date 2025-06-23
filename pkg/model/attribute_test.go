package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttributeString(t *testing.T) {
	t.Run("string_conversion", func(t *testing.T) {
		as := AttributeString("test_attribute")
		assert.Equal(t, "test_attribute", as.String())
	})

	t.Run("empty_string", func(t *testing.T) {
		as := AttributeString("")
		assert.Equal(t, "", as.String())
	})

	t.Run("name_attribute_constant", func(t *testing.T) {
		assert.Equal(t, "name", NameAttribute.String())
	})

	t.Run("entity_constants", func(t *testing.T) {
		assert.Equal(t, "resource_attribute", ResourceAttributeEntity.String())
		assert.Equal(t, "subject_attribute", SubjectAttributeEntity.String())
	})
}

func TestAttribute_Init(t *testing.T) {
	t.Run("basic_initialization", func(t *testing.T) {
		attr := &Attribute{}
		props := map[string]string{
			"type":        "string",
			"description": "User email attribute",
		}

		attr.Init("user-email", ResourceAttributeEntity, props)

		require.NotNil(t, attr.Entity)
		assert.Equal(t, "user-email", attr.Entity.Name)
		assert.Equal(t, ResourceAttributeEntity, attr.Entity.Type)
		assert.NotEmpty(t, attr.Entity.HashID)
		assert.Equal(t, attr.Entity.HashID, attr.EntityID)
		assert.NotNil(t, attr.Assignments)
		assert.NotNil(t, attr.Associations)
		assert.Len(t, attr.Assignments, 0)
		assert.Len(t, attr.Associations, 0)
	})

	t.Run("subject_attribute_initialization", func(t *testing.T) {
		attr := &Attribute{}
		props := map[string]string{
			"category": "identity",
			"required": "true",
		}

		attr.Init("user-role", SubjectAttributeEntity, props)

		require.NotNil(t, attr.Entity)
		assert.Equal(t, "user-role", attr.Entity.Name)
		assert.Equal(t, SubjectAttributeEntity, attr.Entity.Type)
		assert.Equal(t, attr.Entity.HashID, attr.EntityID)
	})

	t.Run("empty_props", func(t *testing.T) {
		attr := &Attribute{}
		props := make(map[string]string)

		attr.Init("empty-attr", ResourceAttributeEntity, props)

		require.NotNil(t, attr.Entity)
		assert.Equal(t, "empty-attr", attr.Entity.Name)
		assert.NotEmpty(t, attr.EntityID)
	})

	t.Run("nil_props", func(t *testing.T) {
		attr := &Attribute{}

		attr.Init("nil-attr", SubjectAttributeEntity, nil)

		require.NotNil(t, attr.Entity)
		assert.Equal(t, "nil-attr", attr.Entity.Name)
		assert.NotEmpty(t, attr.EntityID)
	})

	t.Run("properties_mapping", func(t *testing.T) {
		attr := &Attribute{}
		props := map[string]string{
			"datatype":    "integer",
			"min_value":   "0",
			"max_value":   "100",
			"description": "User age attribute",
		}

		attr.Init("user-age", ResourceAttributeEntity, props)

		// Note: Properties mapping would need the MapToProperty function to be tested properly
		// Here we just verify the structure is initialized
		require.NotNil(t, attr.Properties)
	})
}

func TestAttribute_DeepCopy(t *testing.T) {
	t.Run("basic_deep_copy", func(t *testing.T) {
		attr := &Attribute{}
		props := map[string]string{
			"type": "boolean",
		}
		attr.Init("test-attr", ResourceAttributeEntity, props)

		// Perform deep copy
		copied := attr.DeepCopy()

		assert.Equal(t, attr.Entity.HashID, copied.GetID())
		assert.Equal(t, attr.Entity.Name, copied.GetName())
		assert.Equal(t, attr.Entity.Type, copied.GetType())
	})

	t.Run("deep_copy_independence", func(t *testing.T) {
		attr := &Attribute{}
		attr.Init("original", ResourceAttributeEntity, nil)
		
		copied := attr.DeepCopy()
		
		// Modify original entity
		attr.Entity.Name = "modified"
		
		// Copy should remain unchanged if it's truly a deep copy
		assert.Equal(t, "original", copied.GetName())
	})
}

func TestAttribute_AddAssignments(t *testing.T) {
	t.Run("add_new_assignments", func(t *testing.T) {
		attr := &Attribute{}
		attr.Init("test-attr", ResourceAttributeEntity, nil)

		assignments := []Relationship{
			{HashID: "rel-1", Type: AssignmentRelationship},
			{HashID: "rel-2", Type: AssignmentRelationship},
		}

		attr.AddAssignments(assignments)

		assert.Len(t, attr.Assignments, 2)
		assert.Equal(t, "rel-1", attr.Assignments[0].HashID)
		assert.Equal(t, "rel-2", attr.Assignments[1].HashID)
	})

	t.Run("avoid_duplicate_assignments", func(t *testing.T) {
		attr := &Attribute{}
		attr.Init("test-attr", ResourceAttributeEntity, nil)

		// Add initial assignments
		assignments1 := []Relationship{
			{HashID: "rel-1", Type: AssignmentRelationship},
			{HashID: "rel-2", Type: AssignmentRelationship},
		}
		attr.AddAssignments(assignments1)

		// Add overlapping assignments
		assignments2 := []Relationship{
			{HashID: "rel-1", Type: AssignmentRelationship}, // Duplicate
			{HashID: "rel-3", Type: AssignmentRelationship}, // New
		}
		attr.AddAssignments(assignments2)

		assert.Len(t, attr.Assignments, 3, "Should have 3 unique assignments")
		
		// Verify we have the expected IDs
		ids := make(map[string]bool)
		for _, rel := range attr.Assignments {
			ids[rel.HashID] = true
		}
		assert.True(t, ids["rel-1"])
		assert.True(t, ids["rel-2"])
		assert.True(t, ids["rel-3"])
	})

	t.Run("add_empty_assignments", func(t *testing.T) {
		attr := &Attribute{}
		attr.Init("test-attr", ResourceAttributeEntity, nil)

		attr.AddAssignments([]Relationship{})

		assert.Len(t, attr.Assignments, 0)
	})

	t.Run("add_to_existing_assignments", func(t *testing.T) {
		attr := &Attribute{}
		attr.Init("test-attr", ResourceAttributeEntity, nil)

		// Manually add some assignments first
		attr.Assignments = []Relationship{
			{HashID: "existing-1", Type: AssignmentRelationship},
		}

		newAssignments := []Relationship{
			{HashID: "new-1", Type: AssignmentRelationship},
			{HashID: "new-2", Type: AssignmentRelationship},
		}

		attr.AddAssignments(newAssignments)

		assert.Len(t, attr.Assignments, 3)
	})
}

func TestAttribute_AddAssociations(t *testing.T) {
	t.Run("add_new_associations", func(t *testing.T) {
		attr := &Attribute{}
		attr.Init("test-attr", ResourceAttributeEntity, nil)

		associations := []Relationship{
			{HashID: "assoc-1", Type: AssociationRelationship},
			{HashID: "assoc-2", Type: AssociationRelationship},
		}

		attr.AddAssociations(associations)

		assert.Len(t, attr.Associations, 2)
		assert.Equal(t, "assoc-1", attr.Associations[0].HashID)
		assert.Equal(t, "assoc-2", attr.Associations[1].HashID)
	})

	t.Run("add_empty_associations", func(t *testing.T) {
		attr := &Attribute{}
		attr.Init("test-attr", ResourceAttributeEntity, nil)

		attr.AddAssociations([]Relationship{})

		assert.Len(t, attr.Associations, 0)
	})

	t.Run("add_to_existing_associations", func(t *testing.T) {
		attr := &Attribute{}
		attr.Init("test-attr", ResourceAttributeEntity, nil)

		// Manually add some associations first
		attr.Associations = []Relationship{
			{HashID: "existing-assoc-1", Type: AssociationRelationship},
		}

		newAssociations := []Relationship{
			{HashID: "new-assoc-1", Type: AssociationRelationship},
			{HashID: "new-assoc-2", Type: AssociationRelationship},
		}

		attr.AddAssociations(newAssociations)

		assert.Len(t, attr.Associations, 3)
		assert.Equal(t, "existing-assoc-1", attr.Associations[0].HashID)
		assert.Equal(t, "new-assoc-1", attr.Associations[1].HashID)
		assert.Equal(t, "new-assoc-2", attr.Associations[2].HashID)
	})

	t.Run("add_duplicate_associations_allowed", func(t *testing.T) {
		// Note: Unlike AddAssignments, AddAssociations doesn't check for duplicates
		attr := &Attribute{}
		attr.Init("test-attr", ResourceAttributeEntity, nil)

		associations := []Relationship{
			{HashID: "assoc-1", Type: AssociationRelationship},
			{HashID: "assoc-1", Type: AssociationRelationship}, // Duplicate
		}

		attr.AddAssociations(associations)

		assert.Len(t, attr.Associations, 2, "AddAssociations allows duplicates")
	})
}

func TestAttribute_Integration(t *testing.T) {
	t.Run("full_attribute_lifecycle", func(t *testing.T) {
		attr := &Attribute{}
		props := map[string]string{
			"type":        "enum",
			"values":      "admin,user,guest",
			"description": "User role attribute",
		}

		// Initialize
		attr.Init("user-role", SubjectAttributeEntity, props)

		// Verify initialization
		require.NotNil(t, attr.Entity)
		assert.Equal(t, "user-role", attr.Entity.Name)
		assert.Equal(t, SubjectAttributeEntity, attr.Entity.Type)

		// Add assignments
		assignments := []Relationship{
			{HashID: "role-assignment-1", Type: AssignmentRelationship},
			{HashID: "role-assignment-2", Type: AssignmentRelationship},
		}
		attr.AddAssignments(assignments)

		// Add associations
		associations := []Relationship{
			{HashID: "role-association-1", Type: AssociationRelationship},
		}
		attr.AddAssociations(associations)

		// Verify final state
		assert.Len(t, attr.Assignments, 2)
		assert.Len(t, attr.Associations, 1)

		// Test deep copy
		copied := attr.DeepCopy()
		assert.Equal(t, attr.Entity.HashID, copied.GetID())
	})
}

// BenchmarkAttribute_Init benchmarks attribute initialization
func BenchmarkAttribute_Init(b *testing.B) {
	props := map[string]string{
		"type":        "string",
		"description": "Benchmark attribute",
		"category":    "identity",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attr := &Attribute{}
		attr.Init("benchmark-attr", ResourceAttributeEntity, props)
	}
}

// BenchmarkAttribute_AddAssignments benchmarks assignment addition
func BenchmarkAttribute_AddAssignments(b *testing.B) {
	attr := &Attribute{}
	attr.Init("test-attr", ResourceAttributeEntity, nil)

	assignments := make([]Relationship, 100)
	for i := 0; i < 100; i++ {
		assignments[i] = Relationship{
			HashID: "rel-" + string(rune('a'+i%26)) + string(rune('0'+i%10)),
			Type:   AssignmentRelationship,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attr.Assignments = []Relationship{} // Reset
		attr.AddAssignments(assignments)
	}
}

// BenchmarkAttribute_AddAssociations benchmarks association addition  
func BenchmarkAttribute_AddAssociations(b *testing.B) {
	attr := &Attribute{}
	attr.Init("test-attr", ResourceAttributeEntity, nil)

	associations := make([]Relationship, 100)
	for i := 0; i < 100; i++ {
		associations[i] = Relationship{
			HashID: "assoc-" + string(rune('a'+i%26)) + string(rune('0'+i%10)),
			Type:   AssociationRelationship,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attr.Associations = []Relationship{} // Reset
		attr.AddAssociations(associations)
	}
}