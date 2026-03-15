package tools

import "database/sql"

// ColumnTypeResolver returns the effective type to use for a scanned column.
type ColumnTypeResolver func(column *sql.ColumnType) string

// ScanRows scans sql.Rows into a slice of column-name keyed maps using default scan behavior.
func ScanRows(rows *sql.Rows) ([]map[string]any, error) {
	return ScanRowsTyped(rows, nil)
}

// ScanRowsTyped scans sql.Rows into a slice of column-name keyed maps, allowing callers
// to override the effective type used for each column.
func ScanRowsTyped(rows *sql.Rows, resolveType ColumnTypeResolver) ([]map[string]any, error) {
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	results := make([]map[string]any, 0)

	for rows.Next() {
		scanArgs := make([]any, len(columnTypes))

		for i, columnType := range columnTypes {
			effectiveType := ""
			if resolveType != nil {
				effectiveType = resolveType(columnType)
			}
			if effectiveType == "" {
				effectiveType = columnType.DatabaseTypeName()
			}

			switch effectiveType {
			case "TEXT":
				scanArgs[i] = new(sql.NullString)
			case "INTEGER":
				scanArgs[i] = new(sql.NullInt64)
			case "REAL":
				scanArgs[i] = new(sql.NullFloat64)
			case "BLOB":
				scanArgs[i] = new(sql.RawBytes)
			default:
				scanArgs[i] = new(any)
			}
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}

		row := make(map[string]any, len(columnTypes))
		for i, columnType := range columnTypes {
			switch value := scanArgs[i].(type) {
			case *sql.NullString:
				if value.Valid {
					row[columnType.Name()] = value.String
				} else {
					row[columnType.Name()] = nil
				}
			case *sql.NullInt64:
				if value.Valid {
					row[columnType.Name()] = value.Int64
				} else {
					row[columnType.Name()] = nil
				}
			case *sql.NullFloat64:
				if value.Valid {
					row[columnType.Name()] = value.Float64
				} else {
					row[columnType.Name()] = nil
				}
			case *sql.RawBytes:
				if value == nil {
					row[columnType.Name()] = nil
				} else {
					cloned := make([]byte, len(*value))
					copy(cloned, *value)
					row[columnType.Name()] = cloned
				}
			case *any:
				row[columnType.Name()] = *value
			default:
				row[columnType.Name()] = value
			}
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
