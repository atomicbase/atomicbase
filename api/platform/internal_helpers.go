package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/atombasedev/atombase/config"
)

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}

func generateSchemaSQL(schema Schema) []string {
	statements := make([]string, 0, len(schema.Tables))
	for _, table := range schema.Tables {
		var columns []string
		for _, col := range table.Columns {
			def := fmt.Sprintf("[%s]", col.Name)
			if contains(table.Pk, col.Name) {
				def += " PRIMARY KEY"
			}
			if col.NotNull {
				def += " NOT NULL"
			}
			if col.Default != nil {
				def += fmt.Sprintf(" DEFAULT %v", col.Default)
			}
			if col.References != "" {
				parts := strings.SplitN(col.References, ".", 2)
				if len(parts) == 2 {
					def += fmt.Sprintf(" REFERENCES [%s]([%s])", parts[0], parts[1])
				}
			}
			columns = append(columns, def)
		}
		statements = append(statements, fmt.Sprintf("CREATE TABLE IF NOT EXISTS [%s] (%s)", table.Name, strings.Join(columns, ", ")))
	}
	return statements
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

var (
	tursoCreateDatabaseFn = tursocreateDatabase
	tursoDeleteDatabaseFn = tursodeleteDatabase
	tursoCreateTokenFn    = tursoCreateToken
)

func tursocreateDatabase(ctx context.Context, name string) error {
	url := fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases", config.Cfg.TursoOrganization)
	body, _ := json.Marshal(map[string]any{"name": name})
	return doTursoJSON(ctx, http.MethodPost, url, body, nil)
}

func tursodeleteDatabase(ctx context.Context, name string) error {
	url := fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s", config.Cfg.TursoOrganization, name)
	return doTursoJSON(ctx, http.MethodDelete, url, nil, nil)
}

func tursoCreateToken(ctx context.Context, name string) (string, error) {
	url := fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s/auth/tokens", config.Cfg.TursoOrganization, name)
	var resp struct {
		JWT string `json:"jwt"`
	}
	if err := doTursoJSON(ctx, http.MethodPost, url, []byte(`{"authorization":"full-access"}`), &resp); err != nil {
		return "", err
	}
	return resp.JWT, nil
}

func doTursoJSON(ctx context.Context, method, url string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.Cfg.TursoAPIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("turso api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
