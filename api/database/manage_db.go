package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// readAPIError reads the error message from a Turso API error response.
func readAPIError(res *http.Response) error {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("turso API error: %s", res.Status)
	}
	if len(body) > 0 {
		return fmt.Errorf("turso API error: %s - %s", res.Status, string(body))
	}
	return fmt.Errorf("turso API error: %s", res.Status)
}

// gets all turso dbs within an organization and stores them
func (dao PrimaryDao) RegisterAllDbs(ctx context.Context) error {
	type dbInfo struct {
		Name     string `json:"Name"`
		Hostname string `json:"Hostname"`
	}

	type databases struct {
		Databases []dbInfo `json:"databases"`
	}

	org := os.Getenv("TURSO_ORGANIZATION")
	if org == "" {
		return errors.New("TURSO_ORGANIZATION is not set but is required for managing turso databases")
	}
	token := os.Getenv("TURSO_API_KEY")
	if token == "" {
		return errors.New("TURSO_API_KEY is not set but is required for managing turso databases")
	}

	var client http.Client

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases", org), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return readAPIError(res)
	}

	var dbs databases
	err = json.NewDecoder(res.Body).Decode(&dbs)
	if err != nil {
		return err
	}

	rows, err := dao.Client.QueryContext(ctx, fmt.Sprintf("SELECT name FROM %s", ReservedTableDatabases))
	if err != nil {
		return err
	}
	defer rows.Close()

	var currDbs []string

	for rows.Next() {
		var name sql.NullString

		err := rows.Scan(&name)
		if err != nil {
			return err
		}
		currDbs = append(currDbs, name.String)
	}

	for _, db := range dbs.Databases {
		exists := false
		for i := 0; i < len(currDbs) && !exists; i++ {
			if db.Name == currDbs[i] {
				exists = true
			}
		}

		if !exists {
			dbToken, err := createDBToken(ctx, db.Name)
			if err != nil {
				return err
			}

			// Use Hostname from API response
			connStr := fmt.Sprintf("libsql://%s?authToken=%s", db.Hostname, dbToken)
			newClient, err := sql.Open("libsql", connStr)
			if err != nil {
				return err
			}
			defer newClient.Close()

			err = newClient.PingContext(ctx)

			if err != nil {
				return err
			}

			tbls, err := schemaCols(newClient)
			if err != nil {
				return err
			}
			fks, err := schemaFks(newClient)
			if err != nil {
				return err
			}
			ftsTables, err := schemaFTS(newClient)
			if err != nil {
				return err
			}

			var buf bytes.Buffer
			schema := SchemaCache{Tables: tbls, Fks: fks, FTSTables: ftsTables}
			enc := gob.NewEncoder(&buf)

			err = enc.Encode(schema)
			if err != nil {
				return err
			}

			_, err = dao.Client.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, token, schema) values (?, ?, ?)", ReservedTableDatabases), db.Name, dbToken, buf.Bytes())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// creates a schema cache and stores it for an already existing turso db
func (dao PrimaryDao) RegisterDB(ctx context.Context, body io.ReadCloser, dbToken string) ([]byte, error) {

	type reqBody struct {
		Name string `json:"name"`
	}

	type dbResponse struct {
		Database struct {
			Hostname string `json:"Hostname"`
		} `json:"database"`
	}

	var bod reqBody

	err := json.NewDecoder(body).Decode(&bod)
	if err != nil {
		return nil, err
	}

	if dbToken == "" {
		dbToken, err = createDBToken(ctx, bod.Name)
		if err != nil {
			return nil, err
		}
	}

	org := os.Getenv("TURSO_ORGANIZATION")
	if org == "" {
		return nil, errors.New("TURSO_ORGANIZATION is not set but is required for managing turso databases")
	}
	token := os.Getenv("TURSO_API_KEY")
	if token == "" {
		return nil, errors.New("TURSO_API_KEY is not set but is required for managing turso databases")
	}

	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s", org, bod.Name), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(request)

	fmt.Println("Made request to turso")
	if err != nil {
		fmt.Println("Fail at request: ", err)
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		fmt.Println("Fail at status code: ", res.StatusCode)
		return nil, readAPIError(res)
	}

	fmt.Println("Received response from turso")

	var dbResp dbResponse
	if err := json.NewDecoder(res.Body).Decode(&dbResp); err != nil {
		return nil, err
	}

	// Use Hostname from API response
	connStr := fmt.Sprintf("libsql://%s?authToken=%s", dbResp.Database.Hostname, dbToken)
	newClient, err := sql.Open("libsql", connStr)
	if err != nil {
		return nil, err
	}
	defer newClient.Close()

	err = newClient.PingContext(ctx)

	if err != nil {
		return nil, err
	}

	tbls, err := schemaCols(newClient)
	if err != nil {
		return nil, err
	}
	fks, err := schemaFks(newClient)
	if err != nil {
		return nil, err
	}
	ftsTables, err := schemaFTS(newClient)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	schema := SchemaCache{Tables: tbls, Fks: fks, FTSTables: ftsTables}
	enc := gob.NewEncoder(&buf)

	err = enc.Encode(schema)
	if err != nil {
		return nil, err
	}

	_, err = dao.Client.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, token, schema) values (?, ?, ?)", ReservedTableDatabases), bod.Name, dbToken, buf.Bytes())

	return []byte(fmt.Sprintf(`{"message":"database %s registered"}`, bod.Name)), err
}

func (dao PrimaryDao) ListDBs(ctx context.Context) ([]byte, error) {
	// Exclude primary database (id=1, name=NULL) from the list
	row := dao.Client.QueryRowContext(ctx, fmt.Sprintf("SELECT json_group_array(json_object('name', name, 'id', id)) AS data FROM (SELECT name, id from %s WHERE name IS NOT NULL ORDER BY id)", ReservedTableDatabases))

	if row.Err() != nil {
		return nil, row.Err()
	}

	var res []byte

	err := row.Scan(&res)

	return res, err
}

// for use with the primary database
func (dao PrimaryDao) CreateDB(ctx context.Context, body io.ReadCloser) ([]byte, error) {
	type reqBody struct {
		Name  string `json:"name"`
		Group string `json:"group"`
	}

	var bod reqBody

	err := json.NewDecoder(body).Decode(&bod)
	if err != nil {
		return nil, err
	}

	if bod.Group == "" {
		bod.Group = "default"
	}

	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(bod)
	if err != nil {
		return nil, err
	}

	org := os.Getenv("TURSO_ORGANIZATION")
	if org == "" {
		return nil, errors.New("TURSO_ORGANIZATION is not set but is required for managing turso databases")
	}
	token := os.Getenv("TURSO_API_KEY")
	if token == "" {
		return nil, errors.New("TURSO_API_KEY is not set but is required for managing turso databases")
	}

	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases", org), &buf)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, readAPIError(res)
	}

	buf.Reset()
	var schema SchemaCache
	enc := gob.NewEncoder(&buf)

	err = enc.Encode(schema)
	if err != nil {
		return nil, err
	}

	newToken, err := createDBToken(ctx, bod.Name)
	if err != nil {
		return nil, err
	}

	_, err = dao.Client.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (name, token, schema) values (?, ?, ?)", ReservedTableDatabases), bod.Name, newToken, buf.Bytes())
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf(`{"message":"database %s created"}`, bod.Name)), nil
}

// for use with the primary database
func (dao PrimaryDao) DeleteDB(ctx context.Context, name string) ([]byte, error) {

	_, err := dao.Client.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE name = ?", ReservedTableDatabases), name)
	if err != nil {
		return nil, err
	}

	org := os.Getenv("TURSO_ORGANIZATION")
	if org == "" {
		return nil, errors.New("TURSO_ORGANIZATION is not set but is required for managing turso databases")
	}
	token := os.Getenv("TURSO_API_KEY")
	if token == "" {
		return nil, errors.New("TURSO_API_KEY is not set but is required for managing turso databases")
	}

	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s", org, name), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, readAPIError(res)
	}

	return []byte(fmt.Sprintf(`{"message":"database %s deleted"}`, name)), nil
}

// createDBToken creates a new auth token for a Turso database.
// If TURSO_TOKEN_EXPIRATION is set (e.g., "7d", "30d", "never"), it will be used.
func createDBToken(ctx context.Context, dbName string) (string, error) {
	fmt.Println("Creating db token")
	type jwtBody struct {
		Jwt string `json:"jwt"`
	}

	org := os.Getenv("TURSO_ORGANIZATION")
	if org == "" {
		return "", errors.New("TURSO_ORGANIZATION is not set but is required for managing turso databases")
	}
	token := os.Getenv("TURSO_API_KEY")
	if token == "" {
		return "", errors.New("TURSO_API_KEY is not set but is required for managing turso databases")
	}

	// Build URL with optional expiration parameter
	url := fmt.Sprintf("https://api.turso.tech/v1/organizations/%s/databases/%s/auth/tokens", org, dbName)
	if expiration := os.Getenv("TURSO_TOKEN_EXPIRATION"); expiration != "" {
		url += "?expiration=" + expiration
	}

	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	fmt.Println("Making request to turso")
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("Fail at request: ", err)
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		fmt.Println("Fail at status code: ", res.StatusCode)
		return "", readAPIError(res)
	}

	var jwtBod jwtBody
	if err := json.NewDecoder(res.Body).Decode(&jwtBod); err != nil {
		return "", err
	}

	return jwtBod.Jwt, nil
}
