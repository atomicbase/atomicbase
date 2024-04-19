package daos

import (
	"net/url"
	"testing"
)

func TestBuildWhere(t *testing.T) {
	var val url.Values = map[string][]string{
		"or":             {"users.name:eq.\"eq.72,\",cars.tires:gte.9"},
		"users.username": {"eq.joe"},
	}

	tbls := make(map[string]map[string]string)
	tbls["users"] = map[string]string{
		"name":     "TEXT",
		"username": "TEXT",
	}
	tbls["cars"] = map[string]string{
		"tires": "TEXT",
	}

	schema := SchemaCache{tbls, nil, nil}

	query, args, err := schema.BuildWhere("users", val)
	if err != nil {
		t.Error(err)
	}

	if len(args) != 3 {
		t.Error("expected three arguments but got ", len(args))
	}

	switch query {
	case "WHERE [users].[name] = ? OR [cars].[tires] >= ? AND [users].[username] = ? ":
		if args[0] != "eq.72," {
			t.Error("expected 1st param to be \"eq.72,\" but got ", args[0])
		}
		if args[1] != "9" {
			t.Error("expected 2nd param to be \"9\" but got ", args[1])
		}
		if args[2] != "joe" {
			t.Error("expected 3rd param to be \"joe\" but got", args[2])
		}
	case "WHERE [users].[username] = ? OR [users].[name] = ? OR  [cars].[tires] >= ?":
		if args[2] != "joe" {
			t.Error("expected 3rd param to be \"joe\" but got", args[2])
		}
		if args[0] != "eq.72," {
			t.Error("expected 1st param to be \"eq.72,\" but got ", args[0])
		}
		if args[1] != "9" {
			t.Error("expected 2nd param to be \"9\" but got ", args[1])
		}
	default:
		t.Error("WHERE query not what expected. got: ", query)
	}

}

func TestBuildOrder(t *testing.T) {

	tbls := make(map[string]map[string]string)
	tbls["users"] = map[string]string{
		"name": "TEXT",
	}
	tbls["cars"] = map[string]string{
		"tires": "TEXT",
	}

	schema := SchemaCache{tbls, nil, nil}

	oStr := "users.name:asc,cars.tires:Desc"

	order, err := schema.BuildOrder("users", oStr)
	if err != nil {
		t.Error(err)
	}

	if order != "ORDER BY [users].[name] ASC, [cars].[tires] DESC" {
		t.Error(err)
	}
}
