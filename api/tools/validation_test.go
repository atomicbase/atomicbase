package tools

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateIdentifier(t *testing.T) {
	longName := strings.Repeat("a", MaxIdentifierLength+1)

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "valid simple", input: "users", wantErr: nil},
		{name: "valid underscore", input: "_internal_1", wantErr: nil},
		{name: "empty", input: "", wantErr: ErrEmptyIdentifier},
		{name: "too long", input: longName, wantErr: ErrIdentifierTooLong},
		{name: "invalid first char", input: "1users", wantErr: ErrInvalidCharacter},
		{name: "invalid later char", input: "user-name", wantErr: ErrInvalidCharacter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIdentifier(tt.input)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateTableAndColumnName(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) error
	}{
		{name: "table", fn: ValidateTableName},
		{name: "column", fn: ValidateColumnName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn("bad-name")
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !errors.Is(err, ErrInvalidCharacter) {
				t.Fatalf("expected wrapped invalid character error, got %v", err)
			}
		})
	}
}

func TestValidateDDLQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr error
	}{
		{name: "create table", query: "CREATE TABLE users (id INTEGER)", wantErr: nil},
		{name: "alter table", query: "  alter table users add column name text", wantErr: nil},
		{name: "drop table", query: "\nDROP TABLE users", wantErr: nil},
		{name: "select rejected", query: "SELECT * FROM users", wantErr: ErrNotDDLQuery},
		{name: "empty rejected", query: "   ", wantErr: ErrNotDDLQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDDLQuery(tt.query)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
