package daos

import "fmt"

func InvalidTypeErr(column, typeName string) error {
	return fmt.Errorf("type %s is not a valid type for column %s", typeName, column)
}
