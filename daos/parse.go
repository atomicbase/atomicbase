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

func mapOperator(str string) string {
	switch str {
	case "eq":
		return "="
	case "lt":
		return "<"
	case "gt":
		return ">"
	case "lte":
		return "<="
	case "gte":
		return ">="
	case "neq":
		return "!="
	case "like":
		return "LIKE"
	case "glob":
		return "GLOB"
	case "between":
		return "BETWEEN"
	case "not":
		return "NOT"
	case "in":
		return "IN"
	case "is":
		return "IS"
	case "and":
		return "AND"
	case "or":
		return "OR"
	default:
		return ""
	}
}
