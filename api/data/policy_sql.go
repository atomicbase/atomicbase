package data

import "github.com/atombasedev/atombase/definitions"

func appendPolicyWhere(where string, args []any, policy definitions.CompiledPredicate) (string, []any) {
	if policy.SQL == "" {
		return where, args
	}
	if where == "" {
		return "WHERE " + policy.SQL + " ", append(args, policy.Args...)
	}
	return where + "AND " + policy.SQL + " ", append(args, policy.Args...)
}
