package definitions

import "testing"

func TestCompiler_OrganizationAuthRoleCompilesToMembershipPredicate(t *testing.T) {
	compiler := NewCompiler()
	policy := &AccessPolicy{
		Condition: &Condition{Field: "auth.role", Op: "eq", Value: "owner"},
	}

	predicate, err := compiler.Compile(policy, CompileInput{
		Principal: Principal{UserID: "user-1", AuthStatus: AuthStatusAuthenticated},
		Target: DatabaseTarget{
			DatabaseID:        "org-db",
			DefinitionID:      1,
			DefinitionType:    DefinitionTypeOrganization,
			DefinitionVersion: 1,
		},
		Table:     "projects",
		Operation: "select",
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	if predicate.SQL == "" {
		t.Fatal("expected SQL predicate for organization role policy")
	}
	if len(predicate.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(predicate.Args))
	}
}

func TestValidateConditionContext_RejectsInvalidScopes(t *testing.T) {
	err := ValidateConditionContext(Condition{Field: "auth.role", Op: "eq", Value: "owner"}, "select", DefinitionTypeGlobal)
	if err == nil {
		t.Fatal("expected auth.role on global definition to fail validation")
	}

	err = ValidateConditionContext(Condition{Field: "new.status", Op: "eq", Value: "draft"}, "delete", DefinitionTypeOrganization)
	if err == nil {
		t.Fatal("expected new.* in delete policy to fail validation")
	}
}
