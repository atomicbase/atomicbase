package daos

import (
	"testing"
)

func TestTokenKeyValList(t *testing.T) {
	str := "users.name\":\":eq.john\\.\\,doe,name,name:neq.776"

	keyVals := tokenKeyValList(str)

	if len(keyVals) != 3 {
		t.Error("expected 3 elements but got ", len(keyVals))
	}

	if keyVals[0][0][0] != "users" {
		t.Error(keyVals[0][0][0])
	}

	if keyVals[0][0][1] != "name:" {
		t.Error(keyVals[0][0][1])
	}

	if keyVals[0][1][0] != "eq" {
		t.Error(keyVals[0][1][0])
	}

	if keyVals[0][1][1] != "john.,doe" {
		t.Error(keyVals[0][1][1])
	}

	if keyVals[1][0][0] != "name" {
		t.Error(keyVals[1][0][0])
	}

	if keyVals[1][1] != nil {
		t.Error(keyVals[1][1][0])
	}

	if keyVals[2][0][0] != "name" {
		t.Error(keyVals[2][0][0])
	}

	if keyVals[2][1][0] != "neq" {
		t.Error(keyVals[2][1][0])
	}

	if keyVals[2][1][1] != "776" {
		t.Error(keyVals[2][1][1])
	}

}

func TestTokenKeyVal(t *testing.T) {

	str := "users.name\":\":eq.john\\.\\,doe"

	keyVal := tokenKeyVal(str)

	if len(keyVal[0]) != 2 {
		t.Error("expected 2 elements but got ", len(keyVal[0]))
	}

	if len(keyVal[1]) != 2 {
		t.Error("expected 2 elements but got ", len(keyVal[1]))
	}

	if keyVal[0][0] != "users" {
		t.Error(keyVal[0][0])
	}

	if keyVal[0][1] != "name:" {
		t.Error(keyVal[0][1])
	}

	if keyVal[1][0] != "eq" {
		t.Error(keyVal[1][0])
	}

	if keyVal[1][1] != "john.,doe" {
		t.Error(keyVal[1][1])
	}
}

func TestTokenList(t *testing.T) {
	str := "t\\\"est.name\".age\"\\.test,tbl.col\",\""

	list := tokenList(str)

	if len(list) != 2 {
		t.Error("expected 2 elements but got ", len(list))
	}

	if len(list[0]) != 2 {
		t.Error("expected 2 tokens but got ", len(list[0]))
	}

	if list[0][0] != "t\"est" {
		t.Error(list[0][0])
	}

	if list[0][1] != "name.age.test" {
		t.Error(list[0][1])
	}

	if list[1][0] != "tbl" {
		t.Error(list[1][0])
	}

	if list[1][1] != "col," {
		t.Error(list[1][1])
	}

}

func TestToken(t *testing.T) {
	str := "t\\\"est.name\".age\"\\.test"

	tokens := token(str)

	if len(tokens) != 2 {
		t.Error("expected 2 tokens but got ", len(tokens))
	}

	if tokens[0] != "t\"est" {
		t.Error(tokens[0])
	}

	if tokens[1] != "name.age.test" {
		t.Error(tokens[1])
	}

}
