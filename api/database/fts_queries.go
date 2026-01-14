package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// FTSIndexRequest represents the request body for creating an FTS index.
type FTSIndexRequest struct {
	Columns []string `json:"columns"`
}

// CreateFTSIndex creates an FTS5 virtual table for full-text search on the specified columns.
// The FTS table is named {table}_fts and uses external content mode to sync with the source table.
func (dao *Database) CreateFTSIndex(ctx context.Context, table string, body io.ReadCloser) ([]byte, error) {
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}

	// Check table exists
	tbl, err := dao.Schema.SearchTbls(table)
	if err != nil {
		return nil, err
	}

	// Check if FTS index already exists
	if dao.Schema.HasFTSIndex(table) {
		return nil, fmt.Errorf("FTS index already exists for table %s", table)
	}

	var req FTSIndexRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return nil, err
	}

	if len(req.Columns) == 0 {
		return nil, fmt.Errorf("at least one column is required for FTS index")
	}

	// Validate all columns exist and are TEXT type
	for _, col := range req.Columns {
		c, err := tbl.SearchCols(col)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(c.Type, ColTypeText) {
			return nil, fmt.Errorf("column %s must be TEXT type for FTS index, got %s", col, c.Type)
		}
	}

	ftsTable := table + FTSSuffix
	columnList := strings.Join(req.Columns, ", ")
	quotedColumns := "[" + strings.Join(req.Columns, "], [") + "]"

	// Build new. and old. prefixed column lists for triggers
	newColumns := "new.[" + strings.Join(req.Columns, "], new.[") + "]"
	oldColumns := "old.[" + strings.Join(req.Columns, "], old.[") + "]"

	// Create FTS5 virtual table with external content
	createFTS := fmt.Sprintf(`
		CREATE VIRTUAL TABLE [%s] USING fts5(
			%s,
			content='%s',
			content_rowid='rowid'
		);
	`, ftsTable, columnList, table)

	// Populate with existing data
	populateFTS := fmt.Sprintf(`
		INSERT INTO [%s](rowid, %s)
		SELECT rowid, %s FROM [%s];
	`, ftsTable, quotedColumns, quotedColumns, table)

	// Create triggers for auto-sync
	insertTrigger := fmt.Sprintf(`
		CREATE TRIGGER [%s_fts_insert] AFTER INSERT ON [%s] BEGIN
			INSERT INTO [%s](rowid, %s) VALUES (new.rowid, %s);
		END;
	`, table, table, ftsTable, quotedColumns, newColumns)

	deleteTrigger := fmt.Sprintf(`
		CREATE TRIGGER [%s_fts_delete] AFTER DELETE ON [%s] BEGIN
			INSERT INTO [%s]([%s], rowid, %s) VALUES('delete', old.rowid, %s);
		END;
	`, table, table, ftsTable, ftsTable, quotedColumns, oldColumns)

	updateTrigger := fmt.Sprintf(`
		CREATE TRIGGER [%s_fts_update] AFTER UPDATE ON [%s] BEGIN
			INSERT INTO [%s]([%s], rowid, %s) VALUES('delete', old.rowid, %s);
			INSERT INTO [%s](rowid, %s) VALUES (new.rowid, %s);
		END;
	`, table, table, ftsTable, ftsTable, quotedColumns, oldColumns, ftsTable, quotedColumns, newColumns)

	// Execute all statements
	query := createFTS + populateFTS + insertTrigger + deleteTrigger + updateTrigger
	_, err = dao.Client.ExecContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to create FTS index: %w", err)
	}

	return []byte(fmt.Sprintf(`{"message":"FTS index created for table %s"}`, table)), dao.InvalidateSchema(ctx)
}

// DropFTSIndex removes the FTS5 virtual table and its associated triggers.
func (dao *Database) DropFTSIndex(ctx context.Context, table string) ([]byte, error) {
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}

	// Check if FTS index exists
	if !dao.Schema.HasFTSIndex(table) {
		return nil, fmt.Errorf("no FTS index exists for table %s", table)
	}

	ftsTable := table + FTSSuffix

	// Drop triggers and FTS table
	query := fmt.Sprintf(`
		DROP TRIGGER IF EXISTS [%s_fts_insert];
		DROP TRIGGER IF EXISTS [%s_fts_delete];
		DROP TRIGGER IF EXISTS [%s_fts_update];
		DROP TABLE IF EXISTS [%s];
	`, table, table, table, ftsTable)

	_, err := dao.Client.ExecContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to drop FTS index: %w", err)
	}

	return []byte(fmt.Sprintf(`{"message":"FTS index dropped for table %s"}`, table)), dao.InvalidateSchema(ctx)
}

// ListFTSIndexes returns a list of all tables that have FTS indexes.
func (dao *Database) ListFTSIndexes() ([]byte, error) {
	type ftsInfo struct {
		Table    string   `json:"table"`
		FTSTable string   `json:"ftsTable"`
		Columns  []string `json:"columns"`
	}

	var indexes []ftsInfo

	for table := range dao.Schema.FTSTables {
		ftsTable := table + FTSSuffix

		// Get columns from FTS table
		var columns []string
		ftsTableInfo, err := dao.Schema.SearchTbls(ftsTable)
		if err == nil {
			for colName, col := range ftsTableInfo.Columns {
				// Skip internal FTS columns
				if !strings.HasPrefix(colName, ftsTable) {
					columns = append(columns, col.Name)
				}
			}
		}

		indexes = append(indexes, ftsInfo{
			Table:    table,
			FTSTable: ftsTable,
			Columns:  columns,
		})
	}

	return json.Marshal(indexes)
}
