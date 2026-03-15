package data

import (
	"context"

	"github.com/atombasedev/atombase/definitions"
)

func (dao *TenantConnection) compilePolicy(ctx context.Context, table, operation string, values map[string]any) (definitions.CompiledPredicate, error) {
	policy, err := dao.primaryStore.LoadAccessPolicy(ctx, dao.DefinitionID, dao.DatabaseVersion, table, operation)
	if err != nil {
		return definitions.CompiledPredicate{}, err
	}

	return definitions.NewCompiler().Compile(policy, definitions.CompileInput{
		Principal: dao.Principal,
		Target: definitions.DatabaseTarget{
			DatabaseID:        dao.ID,
			DefinitionID:      dao.DefinitionID,
			DefinitionType:    dao.DefinitionType,
			DefinitionVersion: dao.DatabaseVersion,
		},
		Table:     table,
		Operation: operation,
		NewValues: values,
	})
}
