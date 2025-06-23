package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssociation_Constants(t *testing.T) {
	t.Run("association_relationship_constant", func(t *testing.T) {
		assert.Equal(t, "association", AssociationRelationship.String())
	})
}

func TestAssociation_Init(t *testing.T) {
	t.Run("basic_initialization", func(t *testing.T) {
		// Create entities
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), map[string]string{"role": "admin"})

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), map[string]string{"type": "document"})

		// Initialize association
		association := &Association{}
		verbs := []string{"read", "write", "delete"}
		className := "admin_access"

		association.Init(*fromEntity, *toEntity, verbs, className)

		require.NotNil(t, association.Relationship)
		assert.Equal(t, fromEntity.HashID, association.Relationship.FromID)
		assert.Equal(t, toEntity.HashID, association.Relationship.ToID)
		assert.Equal(t, AssociationRelationship, association.Relationship.Type)
		assert.Equal(t, association.Relationship.HashID, association.RelationshipID)
		assert.Equal(t, className, association.ClassName)
		assert.Equal(t, verbs, []string(association.Verbs))
	})

	t.Run("empty_verbs", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		association := &Association{}
		association.Init(*fromEntity, *toEntity, []string{}, "empty_verbs_class")

		require.NotNil(t, association.Relationship)
		assert.Len(t, association.Verbs, 0)
		assert.Equal(t, "empty_verbs_class", association.ClassName)
	})

	t.Run("nil_verbs", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		association := &Association{}
		association.Init(*fromEntity, *toEntity, nil, "nil_verbs_class")

		require.NotNil(t, association.Relationship)
		assert.Nil(t, association.Verbs)
		assert.Equal(t, "nil_verbs_class", association.ClassName)
	})

	t.Run("single_verb", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		association := &Association{}
		verbs := []string{"read"}
		association.Init(*fromEntity, *toEntity, verbs, "read_only")

		assert.Len(t, association.Verbs, 1)
		assert.Equal(t, "read", association.Verbs[0])
	})

	t.Run("multiple_verbs", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("Admin", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("System", EntityString("resource"), nil)

		association := &Association{}
		verbs := []string{"create", "read", "update", "delete", "execute", "manage"}
		association.Init(*fromEntity, *toEntity, verbs, "full_access")

		assert.Len(t, association.Verbs, 6)
		assert.Contains(t, []string(association.Verbs), "create")
		assert.Contains(t, []string(association.Verbs), "read")
		assert.Contains(t, []string(association.Verbs), "update")
		assert.Contains(t, []string(association.Verbs), "delete")
		assert.Contains(t, []string(association.Verbs), "execute")
		assert.Contains(t, []string(association.Verbs), "manage")
	})

	t.Run("empty_class_name", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		association := &Association{}
		association.Init(*fromEntity, *toEntity, []string{"read"}, "")

		assert.Equal(t, "", association.ClassName)
		assert.Len(t, association.Verbs, 1)
	})

	t.Run("consistent_relationship_generation", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), map[string]string{"dept": "eng"})

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), map[string]string{"type": "file"})

		association1 := &Association{}
		association1.Init(*fromEntity, *toEntity, []string{"read"}, "class1")

		association2 := &Association{}
		association2.Init(*fromEntity, *toEntity, []string{"read"}, "class1")

		// Relationship IDs should be the same for same entities
		assert.Equal(t, association1.RelationshipID, association2.RelationshipID)
	})
}

func TestAssociation_DeepCopy(t *testing.T) {
	t.Run("basic_deep_copy", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		association := &Association{}
		association.Init(*fromEntity, *toEntity, []string{"read", "write"}, "test_class")

		// Perform deep copy
		copied := association.DeepCopy()

		assert.Equal(t, association.Relationship.HashID, copied.GetID())
		assert.Equal(t, association.Relationship.FromID, copied.FromID)
		assert.Equal(t, association.Relationship.ToID, copied.ToID)
		assert.Equal(t, association.Relationship.Type, copied.Type)
	})

	t.Run("deep_copy_independence", func(t *testing.T) {
		fromEntity := &Entity{}
		fromEntity.Init("User", EntityString("user"), nil)

		toEntity := &Entity{}
		toEntity.Init("Resource", EntityString("resource"), nil)

		association := &Association{}
		association.Init(*fromEntity, *toEntity, []string{"read"}, "original_class")

		copied := association.DeepCopy()

		// Modify original relationship
		association.Relationship.FromID = "modified_from_id"

		// Copy should remain unchanged if it's truly a deep copy
		assert.NotEqual(t, "modified_from_id", copied.FromID)
	})
}

func TestAssociation_AddVerbs(t *testing.T) {
	t.Run("add_new_verbs", func(t *testing.T) {
		association := &Association{
			Verbs: []string{"read"},
		}

		newVerbs := []string{"write", "delete"}
		association.AddVerbs(newVerbs)

		assert.Len(t, association.Verbs, 3)
		assert.Contains(t, []string(association.Verbs), "read")
		assert.Contains(t, []string(association.Verbs), "write")
		assert.Contains(t, []string(association.Verbs), "delete")
	})

	t.Run("avoid_duplicate_verbs", func(t *testing.T) {
		association := &Association{
			Verbs: []string{"read", "write"},
		}

		newVerbs := []string{"read", "execute"} // "read" is duplicate
		association.AddVerbs(newVerbs)

		assert.Len(t, association.Verbs, 3, "Should have 3 unique verbs")
		
		verbMap := make(map[string]bool)
		for _, verb := range association.Verbs {
			verbMap[verb] = true
		}
		assert.True(t, verbMap["read"])
		assert.True(t, verbMap["write"])
		assert.True(t, verbMap["execute"])
	})

	t.Run("add_empty_verbs", func(t *testing.T) {
		association := &Association{
			Verbs: []string{"read"},
		}

		association.AddVerbs([]string{})

		assert.Len(t, association.Verbs, 1)
		assert.Equal(t, "read", association.Verbs[0])
	})

	t.Run("add_to_empty_verbs", func(t *testing.T) {
		association := &Association{
			Verbs: []string{},
		}

		newVerbs := []string{"create", "read", "update"}
		association.AddVerbs(newVerbs)

		assert.Len(t, association.Verbs, 3)
		assert.Contains(t, []string(association.Verbs), "create")
		assert.Contains(t, []string(association.Verbs), "read")
		assert.Contains(t, []string(association.Verbs), "update")
	})

	t.Run("add_to_nil_verbs", func(t *testing.T) {
		association := &Association{
			Verbs: nil,
		}

		newVerbs := []string{"read", "write"}
		association.AddVerbs(newVerbs)

		assert.Len(t, association.Verbs, 2)
		assert.Contains(t, []string(association.Verbs), "read")
		assert.Contains(t, []string(association.Verbs), "write")
	})

	t.Run("add_all_duplicate_verbs", func(t *testing.T) {
		association := &Association{
			Verbs: []string{"read", "write", "delete"},
		}

		// Try to add verbs that already exist
		duplicateVerbs := []string{"read", "write", "delete"}
		association.AddVerbs(duplicateVerbs)

		assert.Len(t, association.Verbs, 3, "Length should remain the same")
	})

	t.Run("add_verbs_with_special_characters", func(t *testing.T) {
		association := &Association{
			Verbs: []string{"read"},
		}

		specialVerbs := []string{"read:metadata", "write:config", "admin/*"}
		association.AddVerbs(specialVerbs)

		assert.Len(t, association.Verbs, 4)
		assert.Contains(t, []string(association.Verbs), "read:metadata")
		assert.Contains(t, []string(association.Verbs), "write:config")
		assert.Contains(t, []string(association.Verbs), "admin/*")
	})

	t.Run("large_number_of_verbs", func(t *testing.T) {
		association := &Association{
			Verbs: []string{},
		}

		// Add many verbs
		var manyVerbs []string
		for i := 0; i < 1000; i++ {
			manyVerbs = append(manyVerbs, "action_"+string(rune('0'+i%10))+string(rune('a'+i%26)))
		}

		association.AddVerbs(manyVerbs)

		assert.Len(t, association.Verbs, 1000)
	})
}

func TestAssociation_Integration(t *testing.T) {
	t.Run("full_association_lifecycle", func(t *testing.T) {
		// Create a complex scenario with user, role, and resource
		user := &Entity{}
		user.Init("alice", EntityString("user"), map[string]string{
			"department": "engineering",
			"level":      "senior",
		})

		resource := &Entity{}
		resource.Init("secure_document", EntityString("document"), map[string]string{
			"classification": "confidential",
			"owner":          "admin",
		})

		// Initialize association
		association := &Association{}
		initialVerbs := []string{"read", "download"}
		association.Init(*user, *resource, initialVerbs, "senior_engineer_access")

		// Verify initialization
		require.NotNil(t, association.Relationship)
		assert.Equal(t, user.HashID, association.Relationship.FromID)
		assert.Equal(t, resource.HashID, association.Relationship.ToID)
		assert.Equal(t, AssociationRelationship, association.Relationship.Type)
		assert.Equal(t, "senior_engineer_access", association.ClassName)
		assert.Len(t, association.Verbs, 2)

		// Add more verbs
		additionalVerbs := []string{"write", "share"}
		association.AddVerbs(additionalVerbs)

		// Verify final state
		assert.Len(t, association.Verbs, 4)
		assert.Contains(t, []string(association.Verbs), "read")
		assert.Contains(t, []string(association.Verbs), "download")
		assert.Contains(t, []string(association.Verbs), "write")
		assert.Contains(t, []string(association.Verbs), "share")

		// Test deep copy
		copied := association.DeepCopy()
		assert.Equal(t, association.Relationship.HashID, copied.GetID())
	})

	t.Run("association_with_self_reference", func(t *testing.T) {
		entity := &Entity{}
		entity.Init("recursive_entity", EntityString("special"), map[string]string{
			"type": "self_referencing",
		})

		association := &Association{}
		association.Init(*entity, *entity, []string{"self_manage"}, "recursive_class")

		assert.Equal(t, entity.HashID, association.Relationship.FromID)
		assert.Equal(t, entity.HashID, association.Relationship.ToID)
		assert.Equal(t, association.Relationship.FromID, association.Relationship.ToID)
	})
}

// BenchmarkAssociation_Init benchmarks association initialization
func BenchmarkAssociation_Init(b *testing.B) {
	fromEntity := &Entity{}
	fromEntity.Init("BenchUser", EntityString("user"), map[string]string{"dept": "eng"})

	toEntity := &Entity{}
	toEntity.Init("BenchResource", EntityString("resource"), map[string]string{"type": "file"})

	verbs := []string{"read", "write", "execute"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		association := &Association{}
		association.Init(*fromEntity, *toEntity, verbs, "benchmark_class")
	}
}

// BenchmarkAssociation_AddVerbs benchmarks verb addition
func BenchmarkAssociation_AddVerbs(b *testing.B) {
	association := &Association{
		Verbs: []string{"read", "write"},
	}

	newVerbs := []string{"delete", "execute", "manage", "configure"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset to original state
		association.Verbs = []string{"read", "write"}
		association.AddVerbs(newVerbs)
	}
}

// BenchmarkAssociation_DeepCopy benchmarks deep copy operation
func BenchmarkAssociation_DeepCopy(b *testing.B) {
	fromEntity := &Entity{}
	fromEntity.Init("User", EntityString("user"), nil)

	toEntity := &Entity{}
	toEntity.Init("Resource", EntityString("resource"), nil)

	association := &Association{}
	association.Init(*fromEntity, *toEntity, []string{"read", "write"}, "test_class")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = association.DeepCopy()
	}
}