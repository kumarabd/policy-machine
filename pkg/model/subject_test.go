package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubjectString(t *testing.T) {
	t.Run("string_conversion", func(t *testing.T) {
		ss := SubjectString("test_subject")
		assert.Equal(t, "test_subject", ss.String())
	})

	t.Run("empty_string", func(t *testing.T) {
		ss := SubjectString("")
		assert.Equal(t, "", ss.String())
	})

	t.Run("agent_constant", func(t *testing.T) {
		assert.Equal(t, "agent", Agent.String())
	})

	t.Run("subject_entity_constant", func(t *testing.T) {
		assert.Equal(t, "subject", SubjectEntity.String())
	})
}

func TestSubject_Init(t *testing.T) {
	t.Run("basic_initialization", func(t *testing.T) {
		subject := &Subject{}
		props := map[string]string{
			"email":      "user@example.com",
			"department": "engineering",
			"role":       "developer",
		}

		subject.Init("alice", props)

		require.NotNil(t, subject.Entity)
		assert.Equal(t, "alice", subject.Entity.Name)
		assert.Equal(t, SubjectEntity, subject.Entity.Type)
		assert.NotEmpty(t, subject.Entity.HashID)
		assert.Equal(t, subject.Entity.HashID, subject.EntityID)
		assert.NotNil(t, subject.Assignments)
		assert.Len(t, subject.Assignments, 0)
		assert.NotNil(t, subject.Properties)
	})

	t.Run("empty_props", func(t *testing.T) {
		subject := &Subject{}
		props := make(map[string]string)

		subject.Init("bob", props)

		require.NotNil(t, subject.Entity)
		assert.Equal(t, "bob", subject.Entity.Name)
		assert.Equal(t, SubjectEntity, subject.Entity.Type)
		assert.NotEmpty(t, subject.EntityID)
		assert.Equal(t, subject.Entity.HashID, subject.EntityID)
	})

	t.Run("nil_props", func(t *testing.T) {
		subject := &Subject{}

		subject.Init("charlie", nil)

		require.NotNil(t, subject.Entity)
		assert.Equal(t, "charlie", subject.Entity.Name)
		assert.Equal(t, SubjectEntity, subject.Entity.Type)
		assert.NotEmpty(t, subject.EntityID)
	})

	t.Run("consistent_hash_generation", func(t *testing.T) {
		subject1 := &Subject{}
		subject2 := &Subject{}
		props := map[string]string{
			"department": "engineering",
			"level":      "senior",
		}

		subject1.Init("alice", props)
		subject2.Init("alice", props)

		assert.Equal(t, subject1.EntityID, subject2.EntityID, "Same inputs should generate same hash")
		assert.Equal(t, subject1.Entity.HashID, subject2.Entity.HashID)
	})

	t.Run("different_names_different_hashes", func(t *testing.T) {
		subject1 := &Subject{}
		subject2 := &Subject{}
		props := map[string]string{
			"department": "engineering",
		}

		subject1.Init("alice", props)
		subject2.Init("bob", props)

		assert.NotEqual(t, subject1.EntityID, subject2.EntityID, "Different names should generate different hashes")
	})

	t.Run("different_props_different_hashes", func(t *testing.T) {
		subject1 := &Subject{}
		subject2 := &Subject{}

		subject1.Init("alice", map[string]string{"dept": "eng"})
		subject2.Init("alice", map[string]string{"dept": "hr"})

		assert.NotEqual(t, subject1.EntityID, subject2.EntityID, "Different props should generate different hashes")
	})

	t.Run("complex_properties", func(t *testing.T) {
		subject := &Subject{}
		props := map[string]string{
			"email":        "complex.user@example.com",
			"full_name":    "Complex User Name",
			"department":   "engineering/security",
			"permissions":  `["read", "write", "admin"]`,
			"metadata":     `{"joined": "2023-01-01", "last_login": "2023-12-01"}`,
			"unicode_name": "用戶測試",
			"special":      "@#$%^&*()",
		}

		subject.Init("complex_user", props)

		require.NotNil(t, subject.Entity)
		assert.Equal(t, "complex_user", subject.Entity.Name)
		assert.Equal(t, SubjectEntity, subject.Entity.Type)
		assert.NotEmpty(t, subject.EntityID)
	})
}

func TestSubject_DeepCopy(t *testing.T) {
	t.Run("basic_deep_copy", func(t *testing.T) {
		subject := &Subject{}
		props := map[string]string{
			"role": "admin",
		}
		subject.Init("test_user", props)

		// Perform deep copy
		copied := subject.DeepCopy()

		assert.Equal(t, subject.Entity.HashID, copied.GetID())
		assert.Equal(t, subject.Entity.Name, copied.GetName())
		assert.Equal(t, subject.Entity.Type, copied.GetType())
	})

	t.Run("deep_copy_independence", func(t *testing.T) {
		subject := &Subject{}
		subject.Init("original_user", nil)
		
		copied := subject.DeepCopy()
		
		// Modify original entity
		subject.Entity.Name = "modified_user"
		
		// Copy should remain unchanged if it's truly a deep copy
		assert.Equal(t, "original_user", copied.GetName())
	})

	t.Run("deep_copy_with_assignments", func(t *testing.T) {
		subject := &Subject{}
		subject.Init("user_with_assignments", nil)

		// Add some assignments
		assignments := []Relationship{
			{HashID: "assignment-1", Type: AssignmentRelationship},
		}
		subject.AddAssignments(assignments)

		// Deep copy should copy the entity, not the full subject structure
		copied := subject.DeepCopy()

		// The copied entity should match the original entity
		assert.Equal(t, subject.Entity.HashID, copied.GetID())
		assert.Equal(t, subject.Entity.Name, copied.GetName())
		assert.Equal(t, subject.Entity.Type, copied.GetType())
	})
}

func TestSubject_AddAssignments(t *testing.T) {
	t.Run("add_new_assignments", func(t *testing.T) {
		subject := &Subject{}
		subject.Init("test_user", nil)

		assignments := []Relationship{
			{HashID: "role-assignment-1", Type: AssignmentRelationship},
			{HashID: "permission-assignment-1", Type: AssignmentRelationship},
		}

		subject.AddAssignments(assignments)

		assert.Len(t, subject.Assignments, 2)
		assert.Equal(t, "role-assignment-1", subject.Assignments[0].HashID)
		assert.Equal(t, "permission-assignment-1", subject.Assignments[1].HashID)
	})

	t.Run("avoid_duplicate_assignments", func(t *testing.T) {
		subject := &Subject{}
		subject.Init("test_user", nil)

		// Add initial assignments
		assignments1 := []Relationship{
			{HashID: "assignment-1", Type: AssignmentRelationship},
			{HashID: "assignment-2", Type: AssignmentRelationship},
		}
		subject.AddAssignments(assignments1)

		// Add overlapping assignments
		assignments2 := []Relationship{
			{HashID: "assignment-1", Type: AssignmentRelationship}, // Duplicate
			{HashID: "assignment-3", Type: AssignmentRelationship}, // New
		}
		subject.AddAssignments(assignments2)

		assert.Len(t, subject.Assignments, 3, "Should have 3 unique assignments")
		
		// Verify we have the expected IDs
		ids := make(map[string]bool)
		for _, rel := range subject.Assignments {
			ids[rel.HashID] = true
		}
		assert.True(t, ids["assignment-1"])
		assert.True(t, ids["assignment-2"])
		assert.True(t, ids["assignment-3"])
	})

	t.Run("add_empty_assignments", func(t *testing.T) {
		subject := &Subject{}
		subject.Init("test_user", nil)

		subject.AddAssignments([]Relationship{})

		assert.Len(t, subject.Assignments, 0)
	})

	t.Run("add_to_existing_assignments", func(t *testing.T) {
		subject := &Subject{}
		subject.Init("test_user", nil)

		// Manually add some assignments first
		subject.Assignments = []Relationship{
			{HashID: "existing-1", Type: AssignmentRelationship},
		}

		newAssignments := []Relationship{
			{HashID: "new-1", Type: AssignmentRelationship},
			{HashID: "new-2", Type: AssignmentRelationship},
		}

		subject.AddAssignments(newAssignments)

		assert.Len(t, subject.Assignments, 3)
	})

	t.Run("large_number_of_assignments", func(t *testing.T) {
		subject := &Subject{}
		subject.Init("power_user", nil)

		// Create many assignments
		assignments := make([]Relationship, 1000)
		for i := 0; i < 1000; i++ {
			assignments[i] = Relationship{
				HashID: "assignment-" + string(rune('0'+i%10)) + string(rune('a'+i%26)),
				Type:   AssignmentRelationship,
			}
		}

		subject.AddAssignments(assignments)

		assert.Len(t, subject.Assignments, 1000)
	})

	t.Run("mixed_relationship_types", func(t *testing.T) {
		subject := &Subject{}
		subject.Init("test_user", nil)

		assignments := []Relationship{
			{HashID: "assignment-1", Type: AssignmentRelationship},
			{HashID: "association-1", Type: AssociationRelationship}, // Different type
		}

		subject.AddAssignments(assignments)

		assert.Len(t, subject.Assignments, 2)
		// Both should be added regardless of type
		assert.Equal(t, AssignmentRelationship, subject.Assignments[0].Type)
		assert.Equal(t, AssociationRelationship, subject.Assignments[1].Type)
	})
}

func TestSubject_Integration(t *testing.T) {
	t.Run("full_subject_lifecycle", func(t *testing.T) {
		subject := &Subject{}
		props := map[string]string{
			"email":      "integration.test@example.com",
			"department": "engineering",
			"level":      "senior",
			"team":       "core-sre",
		}

		// Initialize
		subject.Init("integration_user", props)

		// Verify initialization
		require.NotNil(t, subject.Entity)
		assert.Equal(t, "integration_user", subject.Entity.Name)
		assert.Equal(t, SubjectEntity, subject.Entity.Type)
		assert.NotEmpty(t, subject.EntityID)

		// Add assignments
		assignments := []Relationship{
			{HashID: "role-developer", Type: AssignmentRelationship},
			{HashID: "team-core-sre", Type: AssignmentRelationship},
			{HashID: "permission-write", Type: AssignmentRelationship},
		}
		subject.AddAssignments(assignments)

		// Verify assignments
		assert.Len(t, subject.Assignments, 3)

		// Test deep copy
		copied := subject.DeepCopy()
		assert.Equal(t, subject.Entity.HashID, copied.GetID())
		assert.Equal(t, subject.Entity.Name, copied.GetName())
		assert.Equal(t, subject.Entity.Type, copied.GetType())

		// Add more assignments
		moreAssignments := []Relationship{
			{HashID: "permission-admin", Type: AssignmentRelationship},
		}
		subject.AddAssignments(moreAssignments)

		// Verify final state
		assert.Len(t, subject.Assignments, 4)
	})

	t.Run("subject_with_agent_pattern", func(t *testing.T) {
		subject := &Subject{}
		props := map[string]string{
			"type":     Agent.String(),
			"version":  "1.0.0",
			"platform": "linux",
		}

		subject.Init("agent-001", props)

		require.NotNil(t, subject.Entity)
		assert.Equal(t, "agent-001", subject.Entity.Name)
		assert.Equal(t, SubjectEntity, subject.Entity.Type)

		// Agent subjects can also have assignments
		assignments := []Relationship{
			{HashID: "agent-role", Type: AssignmentRelationship},
		}
		subject.AddAssignments(assignments)

		assert.Len(t, subject.Assignments, 1)
	})
}

// BenchmarkSubject_Init benchmarks subject initialization
func BenchmarkSubject_Init(b *testing.B) {
	props := map[string]string{
		"email":      "benchmark@example.com",
		"department": "engineering",
		"role":       "developer",
		"team":       "backend",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		subject := &Subject{}
		subject.Init("benchmark_user", props)
	}
}

// BenchmarkSubject_AddAssignments benchmarks assignment addition
func BenchmarkSubject_AddAssignments(b *testing.B) {
	subject := &Subject{}
	subject.Init("benchmark_user", nil)

	assignments := make([]Relationship, 100)
	for i := 0; i < 100; i++ {
		assignments[i] = Relationship{
			HashID: "assignment-" + string(rune('a'+i%26)) + string(rune('0'+i%10)),
			Type:   AssignmentRelationship,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		subject.Assignments = []Relationship{} // Reset
		subject.AddAssignments(assignments)
	}
}

// BenchmarkSubject_DeepCopy benchmarks deep copy operation
func BenchmarkSubject_DeepCopy(b *testing.B) {
	subject := &Subject{}
	props := map[string]string{
		"email": "benchmark@example.com",
		"dept":  "engineering",
	}
	subject.Init("benchmark_user", props)

	// Add some assignments to make it more realistic
	assignments := []Relationship{
		{HashID: "role-1", Type: AssignmentRelationship},
		{HashID: "permission-1", Type: AssignmentRelationship},
	}
	subject.AddAssignments(assignments)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = subject.DeepCopy()
	}
}