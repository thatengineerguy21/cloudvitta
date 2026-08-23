package databaseenginemap

import (
	"errors"
	"testing"
)

func TestMapAWSEngine(t *testing.T) {
	tests := []struct {
		rawEngine  string
		wantEngine string
		wantErr    error
	}{
		{"PostgreSQL", "postgresql", nil},
		{"aurora-postgresql", "postgresql", nil},
		{"Aurora PostgreSQL", "postgresql", nil},
		{"MySQL", "mysql", nil},
		{"Aurora MySQL", "mysql", nil},
		{"SQL Server", "sqlserver", nil},
		{"MariaDB", "mariadb", nil},
		{"Oracle", "oracle", nil},
		{"UnknownDB", "", ErrUnmappedDatabaseEngine},
	}

	for _, tt := range tests {
		got, err := MapAWSEngine(tt.rawEngine)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAWSEngine(%q) error = %v, want %v", tt.rawEngine, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantEngine {
				t.Errorf("MapAWSEngine(%q) = (%q, %v), want (%q, nil)", tt.rawEngine, got, err, tt.wantEngine)
			}
		}
	}
}

func TestMapAzureEngine(t *testing.T) {
	tests := []struct {
		rawEngine  string
		wantEngine string
		wantErr    error
	}{
		{"Azure Database for PostgreSQL", "postgresql", nil},
		{"PostgreSQL Flexible Server", "postgresql", nil},
		{"Azure Database for MySQL", "mysql", nil},
		{"SQL Database", "sqlserver", nil},
		{"Azure SQL Database", "sqlserver", nil},
		{"Azure Database for MariaDB", "mariadb", nil},
		{"UnknownDB", "", ErrUnmappedDatabaseEngine},
	}

	for _, tt := range tests {
		got, err := MapAzureEngine(tt.rawEngine)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapAzureEngine(%q) error = %v, want %v", tt.rawEngine, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantEngine {
				t.Errorf("MapAzureEngine(%q) = (%q, %v), want (%q, nil)", tt.rawEngine, got, err, tt.wantEngine)
			}
		}
	}
}

func TestMapGCPEngine(t *testing.T) {
	tests := []struct {
		rawEngine  string
		wantEngine string
		wantErr    error
	}{
		{"Cloud SQL for PostgreSQL", "postgresql", nil},
		{"AlloyDB for PostgreSQL", "postgresql", nil},
		{"POSTGRES_15", "postgresql", nil},
		{"Cloud SQL for MySQL", "mysql", nil},
		{"MYSQL_8_0", "mysql", nil},
		{"Cloud SQL for SQL Server", "sqlserver", nil},
		{"SQLSERVER_2019_STANDARD", "sqlserver", nil},
		{"UnknownDB", "", ErrUnmappedDatabaseEngine},
	}

	for _, tt := range tests {
		got, err := MapGCPEngine(tt.rawEngine)
		if tt.wantErr != nil {
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("MapGCPEngine(%q) error = %v, want %v", tt.rawEngine, err, tt.wantErr)
			}
		} else {
			if err != nil || got != tt.wantEngine {
				t.Errorf("MapGCPEngine(%q) = (%q, %v), want (%q, nil)", tt.rawEngine, got, err, tt.wantEngine)
			}
		}
	}
}

func TestNormalizeEngine(t *testing.T) {
	got, err := NormalizeEngine("aws", "PostgreSQL")
	if err != nil || got != "postgresql" {
		t.Errorf("NormalizeEngine(aws, PostgreSQL) = (%q, %v), want (postgresql, nil)", got, err)
	}

	_, err = NormalizeEngine("unmapped_provider", "PostgreSQL")
	if err == nil {
		t.Errorf("NormalizeEngine(unmapped_provider, PostgreSQL) expected error, got nil")
	}
}
