package daos

import "strings"

func tokenKeyValList(s string) [][2][]string {
	inQuotes := false
	escaped := false
	list := strings.FieldsFunc(s, func(r rune) bool {
		if escaped {
			escaped = false
			return false
		}

		switch r {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				return true
			}
		}

		return false
	})

	lists := make([][2][]string, len(list))

	for i, val := range list {
		keyVal := tokenKeyVal(val)

		lists[i] = [2][]string{keyVal[0], keyVal[1]}
	}

	return lists
}

func tokenKeyVal(s string) [2][]string {
	inQuotes := false
	escaped := false

	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}

		switch r {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
		case ':':
			if !inQuotes {
				return [2][]string{token(s[:i]), token(s[i+1:])}
			}
		}
	}

	return [2][]string{token(s), nil}
}

func tokenList(s string) [][]string {
	inQuotes := false
	escaped := false
	list := strings.FieldsFunc(s, func(r rune) bool {
		if escaped {
			escaped = false
			return false
		}

		switch r {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				return true
			}
		}

		return false
	})

	lists := make([][]string, len(list))

	for i, val := range list {
		lists[i] = token(val)
	}

	return lists
}

func token(s string) []string {
	inQuotes := false
	escaped := false
	currStr := ""
	var list []string

	for _, r := range s {
		if escaped {
			escaped = false
			currStr += string(r)
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
		case '.':
			if inQuotes {
				currStr += string(r)
			} else {
				list = append(list, currStr)
				currStr = ""
			}
		default:
			currStr += string(r)
		}
	}

	if currStr != "" {
		list = append(list, currStr)
	}

	return list
}

// mapOperator converts a filter operator string to its SQL equivalent.
// Returns empty string if the operator is not recognized.
func mapOperator(str string) string {
	switch str {
	case OpEq:
		return SqlEq
	case OpLt:
		return SqlLt
	case OpGt:
		return SqlGt
	case OpLte:
		return SqlLte
	case OpGte:
		return SqlGte
	case OpNeq:
		return SqlNeq
	case OpLike:
		return SqlLike
	case OpGlob:
		return SqlGlob
	case OpBetween:
		return SqlBetween
	case OpNot:
		return SqlNot
	case OpIn:
		return SqlIn
	case OpIs:
		return SqlIs
	case OpAnd:
		return SqlAnd
	case OpOr:
		return SqlOr
	default:
		return ""
	}
}
