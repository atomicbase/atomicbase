package daos

import "fmt"

func (schema SchemaCache) checkCol(tblName, colName string) error {
	if colName == "*" || colName == "count" {
		if schema.Tables[tblName] != nil {
			return nil
		}

		return fmt.Errorf("table %s does not exist in the schema cache. Your schema cache may be stale", tblName)
	}

	if schema.Tables[tblName][colName] == "" {
		return fmt.Errorf("column %s does not exist on table %s in the schema cache. Your schema cache may be stale", colName, tblName)
	}

	return nil
}

func (schema SchemaCache) checkTbl(tblName string) error {
	if schema.Tables[tblName] == nil {
		return fmt.Errorf("table %s does not exist in the schema cache. Your schema cache may be stale", tblName)
	}

	return nil
}

func InvalidTypeErr(column, typeName string) error {
	return fmt.Errorf("type %s is not a valid type for column %s", typeName, column)
}
