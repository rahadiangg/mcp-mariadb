package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"github.com/rahadiangg/mcp-mariadb/internal/database"
)

// Column represents a column in a table schema
type Column struct {
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Key      string `json:"key"`
	Default  string `json:"default"`
	Extra    string `json:"extra"`
	ForeignKey *ForeignKey `json:"foreign_key,omitempty"`
}

// ForeignKey represents a foreign key relationship
type ForeignKey struct {
	ConstraintName   string `json:"constraint_name"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
	OnUpdate         string `json:"on_update"`
	OnDelete         string `json:"on_delete"`
}

// SchemaWithRelations is the result of get_table_schema_with_relations
type SchemaWithRelations struct {
	TableName string            `json:"table_name"`
	Columns   map[string]Column `json:"columns"`
}

// ValidationResult is the result of validateIdentifier
type ValidationResult struct {
	Valid bool
	Error string
}

// validateIdentifier checks if a string is a valid SQL identifier
func validateIdentifier(name string) ValidationResult {
	if name == "" {
		return ValidationResult{Valid: false, Error: "empty identifier"}
	}
	// Check if identifier is valid (alphanumeric, underscore, not starting with digit)
	for i, r := range name {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return ValidationResult{Valid: false, Error: fmt.Sprintf("identifier '%s' must start with letter or underscore", name)}
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return ValidationResult{Valid: false, Error: fmt.Sprintf("identifier '%s' contains invalid character: %c", name, r)}
		}
	}
	return ValidationResult{Valid: true}
}

// stripComments removes SQL comments from a query
func stripComments(query string) string {
	// Remove single-line comments (-- comment)
	re := regexp.MustCompile(`--.*?$`)
	query = re.ReplaceAllString(query, "")

	// Remove multi-line comments (/* comment */)
	re = regexp.MustCompile(`/\*.*?\*/`)
	query = re.ReplaceAllString(query, "")

	return strings.TrimSpace(query)
}

// isReadOnlyQuery checks if a query is read-only
func isReadOnlyQuery(query string) bool {
	queryUpper := strings.ToUpper(stripComments(query))
	allowedPrefixes := []string{"SELECT", "SHOW", "DESC", "DESCRIBE", "USE", "EXPLAIN"}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(queryUpper, prefix) {
			return true
		}
	}
	return false
}

// listDatabases lists all accessible databases
func (s *Server) listDatabases(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	s.logger.Info("TOOL START: list_databases called")

	var databases []string
	query := "SHOW DATABASES"

	err := s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.SelectContext(ctx, &databases, query)
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: list_databases failed", zap.Error(err))
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	// Filter out system databases if needed
	var result []string
	for _, db := range databases {
		// Skip common system databases
		if db != "information_schema" && db != "mysql" && db != "performance_schema" && db != "sys" {
			result = append(result, db)
		}
	}

	s.logger.Info("TOOL END: list_databases completed", zap.Int("count", len(result)))
	return result, nil
}

// listTables lists all tables in a database
func (s *Server) listTables(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}

	if v := validateIdentifier(databaseName); !v.Valid {
		return nil, fmt.Errorf("invalid database_name: %s", v.Error)
	}

	s.logger.Info("TOOL START: list_tables called", zap.String("database", databaseName))

	var tables []string
	query := "SHOW TABLES"

	err := s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		// Switch to the target database
		if _, err := conn.ExecContext(ctx, "USE `"+databaseName+"`"); err != nil {
			return err
		}
		return conn.SelectContext(ctx, &tables, query)
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: list_tables failed", zap.String("database", databaseName), zap.Error(err))
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	s.logger.Info("TOOL END: list_tables completed", zap.String("database", databaseName), zap.Int("count", len(tables)))
	return tables, nil
}

// getTableSchema retrieves the schema for a specific table
func (s *Server) getTableSchema(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}
	tableName, ok := args["table_name"].(string)
	if !ok {
		return nil, fmt.Errorf("table_name is required")
	}

	if v := validateIdentifier(databaseName); !v.Valid {
		return nil, fmt.Errorf("invalid database_name: %s", v.Error)
	}
	if v := validateIdentifier(tableName); !v.Valid {
		return nil, fmt.Errorf("invalid table_name: %s", v.Error)
	}

	s.logger.Info("TOOL START: get_table_schema called",
		zap.String("database", databaseName),
		zap.String("table", tableName),
	)

	// Query DESCRIBE result
	type describeRow struct {
		Field      string `db:"Field"`
		Type       string `db:"Type"`
		Null       string `db:"Null"`
		Key        string `db:"Key"`
		Default    *string `db:"Default"`
		Extra      string `db:"Extra"`
	}

	var rows []describeRow
	query := fmt.Sprintf("DESCRIBE `%s`.`%s`", databaseName, tableName)

	err := s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.SelectContext(ctx, &rows, query)
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: get_table_schema failed",
			zap.String("database", databaseName),
			zap.String("table", tableName),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get table schema: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("table '%s.%s' not found or has no columns", databaseName, tableName)
	}

	schema := make(map[string]Column)
	for _, row := range rows {
		defaultVal := ""
		if row.Default != nil {
			defaultVal = *row.Default
		}
		schema[row.Field] = Column{
			Type:     row.Type,
			Nullable: strings.ToUpper(row.Null) == "YES",
			Key:      row.Key,
			Default:  defaultVal,
			Extra:    row.Extra,
		}
	}

	s.logger.Info("TOOL END: get_table_schema completed",
		zap.String("database", databaseName),
		zap.String("table", tableName),
		zap.Int("columns", len(schema)))
	return schema, nil
}

// getTableSchemaWithRelations retrieves table schema with foreign key relationships
func (s *Server) getTableSchemaWithRelations(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}
	tableName, ok := args["table_name"].(string)
	if !ok {
		return nil, fmt.Errorf("table_name is required")
	}

	if v := validateIdentifier(databaseName); !v.Valid {
		return nil, fmt.Errorf("invalid database_name: %s", v.Error)
	}
	if v := validateIdentifier(tableName); !v.Valid {
		return nil, fmt.Errorf("invalid table_name: %s", v.Error)
	}

	s.logger.Info("TOOL START: get_table_schema_with_relations called",
		zap.String("database", databaseName),
		zap.String("table", tableName),
	)

	// Get basic schema first
	basicSchema, err := s.getTableSchema(ctx, args)
	if err != nil {
		return nil, err
	}

	schemaMap := basicSchema.(map[string]Column)

	// Query foreign key information
	type fkRow struct {
		ColumnName          string `db:"COLUMN_NAME"`
		ConstraintName      string `db:"CONSTRAINT_NAME"`
		ReferencedTable     string `db:"REFERENCED_TABLE_NAME"`
		ReferencedColumn    string `db:"REFERENCED_COLUMN_NAME"`
		OnUpdate            string `db:"UPDATE_RULE"`
		OnDelete            string `db:"DELETE_RULE"`
	}

	var fkRows []fkRow
	fkQuery := `
		SELECT
			kcu.COLUMN_NAME,
			kcu.CONSTRAINT_NAME,
			kcu.REFERENCED_TABLE_NAME,
			kcu.REFERENCED_COLUMN_NAME,
			rc.UPDATE_RULE,
			rc.DELETE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		INNER JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
			ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
			AND kcu.CONSTRAINT_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.TABLE_SCHEMA = ?
		  AND kcu.TABLE_NAME = ?
		  AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY kcu.CONSTRAINT_NAME, kcu.ORDINAL_POSITION
	`

	err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.SelectContext(ctx, &fkRows, fkQuery, databaseName, tableName)
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: get_table_schema_with_relations failed (FK query)",
			zap.String("database", databaseName),
			zap.String("table", tableName),
			zap.Error(err))
		// Continue without FK info rather than failing completely
		fkRows = []fkRow{}
	}

	// Add FK info to columns
	for colName := range schemaMap {
		col := schemaMap[colName]
		col.ForeignKey = nil
		schemaMap[colName] = col
	}

	for _, fk := range fkRows {
		col, ok := schemaMap[fk.ColumnName]
		if ok {
			col.ForeignKey = &ForeignKey{
				ConstraintName:   fk.ConstraintName,
				ReferencedTable:  fk.ReferencedTable,
				ReferencedColumn: fk.ReferencedColumn,
				OnUpdate:         fk.OnUpdate,
				OnDelete:         fk.OnDelete,
			}
			schemaMap[fk.ColumnName] = col
		}
	}

	result := SchemaWithRelations{
		TableName: tableName,
		Columns:   schemaMap,
	}

	s.logger.Info("TOOL END: get_table_schema_with_relations completed",
		zap.String("database", databaseName),
		zap.String("table", tableName),
		zap.Int("columns", len(schemaMap)),
		zap.Int("foreign_keys", len(fkRows)))
	return result, nil
}

// executeSQL executes a read-only SQL query
func (s *Server) executeSQL(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	sqlQuery, ok := args["sql_query"].(string)
	if !ok {
		return nil, fmt.Errorf("sql_query is required")
	}
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}

	// Validate database name if provided
	if databaseName != "" {
		if v := validateIdentifier(databaseName); !v.Valid {
			return nil, fmt.Errorf("invalid database_name: %s", v.Error)
		}
	}

	// Check for multi-statement injection attempts
	if err := database.ValidateMultiStatementsDisabled(sqlQuery); err != nil {
		s.logger.Warn("Multi-stement query blocked", zap.Error(err))
		return nil, fmt.Errorf("query blocked: %w", err)
	}

	// Check read-only mode
	if s.cfg.ReadOnly && !isReadOnlyQuery(sqlQuery) {
		queryPrefix := sqlQuery
		if len(queryPrefix) > 50 {
			queryPrefix = queryPrefix[:50]
		}
		s.logger.Warn("Non-read-only query blocked in read-only mode",
			zap.String("query_prefix", queryPrefix))
		return nil, fmt.Errorf("operation forbidden: server is in read-only mode")
	}

	queryLog := sqlQuery
	if len(queryLog) > 100 {
		queryLog = queryLog[:100]
	}
	s.logger.Info("TOOL START: execute_sql called",
		zap.String("database", databaseName),
		zap.String("query", queryLog))

	// Extract parameters if provided
	var params []interface{}
	if paramsRaw, ok := args["parameters"].([]interface{}); ok {
		params = paramsRaw
	}

	// Convert parameter placeholders from %s to ? for MySQL driver
	query := sqlQuery
	query = regexp.MustCompile(`%s`).ReplaceAllString(query, "?")

	var results []map[string]interface{}

	err := s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		// Switch to target database if specified
		if databaseName != "" {
			if _, err := conn.ExecContext(ctx, "USE `"+databaseName+"`"); err != nil {
				return fmt.Errorf("failed to switch database: %w", err)
			}
		}

		// Execute query
		rows, err := conn.QueryxContext(ctx, query, params...)
		if err != nil {
			return err
		}
		defer rows.Close()

		// Scan all rows
		for rows.Next() {
			row := make(map[string]interface{})
			if err := rows.MapScan(row); err != nil {
				return err
			}
			results = append(results, row)
		}
		return rows.Err()
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: execute_sql failed",
			zap.String("database", databaseName),
			zap.Error(err))
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	s.logger.Info("TOOL END: execute_sql completed",
		zap.String("database", databaseName),
		zap.Int("rows", len(results)))
	return results, nil
}

// createDatabase creates a new database
func (s *Server) createDatabase(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}

	if v := validateIdentifier(databaseName); !v.Valid {
		return nil, fmt.Errorf("invalid database_name: %s", v.Error)
	}

	s.logger.Info("TOOL START: create_database called", zap.String("database", databaseName))

	// Check if database exists
	var exists bool
	checkQuery := "SELECT COUNT(*) > 0 FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?"
	err := s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.GetContext(ctx, &exists, checkQuery, databaseName)
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: create_database failed (check)", zap.String("database", databaseName), zap.Error(err))
		return nil, fmt.Errorf("failed to check database existence: %w", err)
	}

	if exists {
		s.logger.Info("TOOL END: create_database - already exists", zap.String("database", databaseName))
		return map[string]interface{}{
			"status":       "exists",
			"message":      fmt.Sprintf("Database '%s' already exists", databaseName),
			"database_name": databaseName,
		}, nil
	}

	// Create database
	createQuery := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", databaseName)
	err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		_, err := conn.ExecContext(ctx, createQuery)
		return err
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: create_database failed", zap.String("database", databaseName), zap.Error(err))
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	s.logger.Info("TOOL END: create_database completed", zap.String("database", databaseName))
	return map[string]interface{}{
		"status":       "success",
		"message":      fmt.Sprintf("Database '%s' created successfully", databaseName),
		"database_name": databaseName,
	}, nil
}

// createVectorStore creates a new vector store table
func (s *Server) createVectorStore(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}
	vectorStoreName, ok := args["vector_store_name"].(string)
	if !ok {
		return nil, fmt.Errorf("vector_store_name is required")
	}

	if v := validateIdentifier(databaseName); !v.Valid {
		return nil, fmt.Errorf("invalid database_name: %s", v.Error)
	}
	if v := validateIdentifier(vectorStoreName); !v.Valid {
		return nil, fmt.Errorf("invalid vector_store_name: %s", v.Error)
	}

	// Get optional parameters
	modelName := ""
	if mn, ok := args["model_name"].(string); ok {
		modelName = mn
	}

	distanceFunction := "cosine" // default
	if df, ok := args["distance_function"].(string); ok {
		df = strings.ToLower(df)
		if df != "cosine" && df != "euclidean" {
			return nil, fmt.Errorf("invalid distance_function: '%s'. Must be 'cosine' or 'euclidean'", df)
		}
		distanceFunction = df
	}

	s.logger.Info("TOOL START: create_vector_store called",
		zap.String("database", databaseName),
		zap.String("store", vectorStoreName),
		zap.String("model", modelName),
		zap.String("distance", distanceFunction),
	)

	// Get embedding dimension
	dimension, err := s.embeddingService.GetDimension(ctx, modelName)
	if err != nil {
		s.logger.Error("TOOL ERROR: create_vector_store failed to get dimension", zap.Error(err))
		return nil, fmt.Errorf("failed to get embedding dimension: %w", err)
	}

	// Ensure database exists
	_, err = s.createDatabase(ctx, map[string]interface{}{"database_name": databaseName})
	if err != nil {
		s.logger.Error("TOOL ERROR: create_vector_store failed to ensure database", zap.Error(err))
		return nil, fmt.Errorf("failed to ensure database exists: %w", err)
	}

	// Check if table exists
	var tableExists bool
	checkQuery := `
		SELECT COUNT(*) > 0
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
	`
	err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.GetContext(ctx, &tableExists, checkQuery, databaseName, vectorStoreName)
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: create_vector_store failed (check table)", zap.Error(err))
		return nil, fmt.Errorf("failed to check table existence: %w", err)
	}

	if tableExists {
		s.logger.Info("TOOL END: create_vector_store - table already exists")
		return map[string]interface{}{
			"status":           "exists",
			"message":          fmt.Sprintf("Vector store '%s' already exists in database '%s'", vectorStoreName, databaseName),
			"database_name":    databaseName,
			"vector_store_name": vectorStoreName,
		}, nil
	}

	// Create table with VECTOR column and index
	distanceSQL := "COSINE"
	if distanceFunction == "euclidean" {
		distanceSQL = "EUCLIDEAN"
	}

	createQuery := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(36) NOT NULL DEFAULT UUID_v7() PRIMARY KEY,
			document TEXT NOT NULL,
			embedding VECTOR(%d) NOT NULL,
			metadata JSON NOT NULL,
			VECTOR INDEX (embedding) DISTANCE=%s
		)
	`, vectorStoreName, dimension, distanceSQL)

	err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		// Switch to target database
		if _, err := conn.ExecContext(ctx, "USE `"+databaseName+"`"); err != nil {
			return err
		}
		_, err := conn.ExecContext(ctx, createQuery)
		return err
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: create_vector_store failed", zap.Error(err))
		return nil, fmt.Errorf("failed to create vector store: %w", err)
	}

	s.logger.Info("TOOL END: create_vector_store completed")
	return map[string]interface{}{
		"status":           "success",
		"message":          fmt.Sprintf("Vector store '%s' created successfully with %s distance", vectorStoreName, distanceSQL),
		"database_name":    databaseName,
		"vector_store_name": vectorStoreName,
	}, nil
}

// listVectorStores lists all vector stores in a database
func (s *Server) listVectorStores(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}

	if v := validateIdentifier(databaseName); !v.Valid {
		return nil, fmt.Errorf("invalid database_name: %s", v.Error)
	}

	s.logger.Info("TOOL START: list_vector_stores called", zap.String("database", databaseName))

	// Check if database exists first
	var dbExists bool
	checkDBQuery := "SELECT COUNT(*) > 0 FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?"
	err := s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.GetContext(ctx, &dbExists, checkDBQuery, databaseName)
	})

	if err != nil || !dbExists {
		s.logger.Info("TOOL END: list_vector_stores - database does not exist")
		return []string{}, nil
	}

	// Query for vector stores (tables with indexed VECTOR column named 'embedding')
	query := `
		SELECT DISTINCT T1.TABLE_NAME
		FROM information_schema.COLUMNS AS T1
		INNER JOIN information_schema.STATISTICS AS T2
			ON T1.TABLE_SCHEMA = T2.TABLE_SCHEMA
			AND T1.TABLE_NAME = T2.TABLE_NAME
			AND T1.COLUMN_NAME = T2.COLUMN_NAME
		WHERE T1.TABLE_SCHEMA = ?
		  AND UPPER(T1.COLUMN_NAME) = 'EMBEDDING'
		  AND UPPER(T1.DATA_TYPE) = 'VECTOR'
		ORDER BY T1.TABLE_NAME
	`

	var stores []string
	err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.SelectContext(ctx, &stores, query, databaseName)
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: list_vector_stores failed", zap.Error(err))
		return nil, fmt.Errorf("failed to list vector stores: %w", err)
	}

	s.logger.Info("TOOL END: list_vector_stores completed", zap.Int("count", len(stores)))
	return stores, nil
}

// deleteVectorStore deletes a vector store table
func (s *Server) deleteVectorStore(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}
	vectorStoreName, ok := args["vector_store_name"].(string)
	if !ok {
		return nil, fmt.Errorf("vector_store_name is required")
	}

	if v := validateIdentifier(databaseName); !v.Valid {
		return nil, fmt.Errorf("invalid database_name: %s", v.Error)
	}
	if v := validateIdentifier(vectorStoreName); !v.Valid {
		return nil, fmt.Errorf("invalid vector_store_name: %s", v.Error)
	}

	s.logger.Info("TOOL START: delete_vector_store called",
		zap.String("database", databaseName),
		zap.String("store", vectorStoreName),
	)

	// Check if database exists
	var dbExists bool
	checkDBQuery := "SELECT COUNT(*) > 0 FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?"
	err := s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.GetContext(ctx, &dbExists, checkDBQuery, databaseName)
	})

	if err != nil || !dbExists {
		return map[string]interface{}{
			"status":           "not_found",
			"message":          fmt.Sprintf("Database '%s' does not exist", databaseName),
			"type":             "database",
		}, nil
	}

	// Check if table exists
	var tableExists bool
	checkTableQuery := `
		SELECT COUNT(*) > 0
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
	`
	err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.GetContext(ctx, &tableExists, checkTableQuery, databaseName, vectorStoreName)
	})

	if err != nil || !tableExists {
		return map[string]interface{}{
			"status":           "not_found",
			"message":          fmt.Sprintf("Vector store '%s' does not exist in database '%s'", vectorStoreName, databaseName),
			"type":             "table",
		}, nil
	}

	// Verify it's a vector store
	var isVectorStore bool
	checkVSQuery := `
		SELECT COUNT(*) > 0
		FROM information_schema.COLUMNS AS T1
		INNER JOIN information_schema.STATISTICS AS T2
			ON T1.TABLE_SCHEMA = T2.TABLE_SCHEMA
			AND T1.TABLE_NAME = T2.TABLE_NAME
			AND T1.COLUMN_NAME = T2.COLUMN_NAME
		WHERE T1.TABLE_SCHEMA = ? AND T1.TABLE_NAME = ?
		  AND T1.COLUMN_NAME = 'embedding'
		  AND UPPER(T1.DATA_TYPE) = 'VECTOR'
	`
	err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		return conn.GetContext(ctx, &isVectorStore, checkVSQuery, databaseName, vectorStoreName)
	})

	if err != nil || !isVectorStore {
		return map[string]interface{}{
			"status": "not_vector_store",
			"message": fmt.Sprintf("Table '%s' is not a valid vector store", vectorStoreName),
		}, nil
	}

	// Drop the table
	dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", vectorStoreName)
	err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		// Switch to target database
		if _, err := conn.ExecContext(ctx, "USE `"+databaseName+"`"); err != nil {
			return err
		}
		_, err := conn.ExecContext(ctx, dropQuery)
		return err
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: delete_vector_store failed", zap.Error(err))
		return map[string]interface{}{
			"status":           "error",
			"message":          fmt.Sprintf("Failed to delete vector store: %v", err),
			"database_name":    databaseName,
			"vector_store_name": vectorStoreName,
		}, nil
	}

	s.logger.Info("TOOL END: delete_vector_store completed")
	return map[string]interface{}{
		"status":           "success",
		"message":          fmt.Sprintf("Vector store '%s' deleted successfully", vectorStoreName),
		"database_name":    databaseName,
		"vector_store_name": vectorStoreName,
	}, nil
}

// insertDocsVectorStore inserts documents into a vector store
func (s *Server) insertDocsVectorStore(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}
	vectorStoreName, ok := args["vector_store_name"].(string)
	if !ok {
		return nil, fmt.Errorf("vector_store_name is required")
	}

	if v := validateIdentifier(databaseName); !v.Valid {
		return nil, fmt.Errorf("invalid database_name: %s", v.Error)
	}
	if v := validateIdentifier(vectorStoreName); !v.Valid {
		return nil, fmt.Errorf("invalid vector_store_name: %s", v.Error)
	}

	// Get documents array
	documentsRaw, ok := args["documents"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("documents must be an array")
	}
	if len(documentsRaw) == 0 {
		return nil, fmt.Errorf("documents must not be empty")
	}

	documents := make([]string, len(documentsRaw))
	for i, d := range documentsRaw {
		doc, ok := d.(string)
		if !ok || doc == "" {
			return nil, fmt.Errorf("documents must be non-empty strings")
		}
		documents[i] = doc
	}

	// Get metadata array (optional)
	var metadataList []map[string]interface{}
	if metadataRaw, ok := args["metadata"].([]interface{}); ok {
		if len(metadataRaw) != len(documents) {
			return nil, fmt.Errorf("metadata must have same length as documents")
		}
		metadataList = make([]map[string]interface{}, len(metadataRaw))
		for i, m := range metadataRaw {
			if mm, ok := m.(map[string]interface{}); ok {
				metadataList[i] = mm
			} else {
				metadataList[i] = map[string]interface{}{}
			}
		}
	} else {
		// Create empty metadata for each document
		metadataList = make([]map[string]interface{}, len(documents))
		for i := range metadataList {
			metadataList[i] = map[string]interface{}{}
		}
	}

	s.logger.Info("TOOL START: insert_docs_vector_store called",
		zap.String("database", databaseName),
		zap.String("store", vectorStoreName),
		zap.Int("documents", len(documents)),
	)

	// Generate embeddings
	embeddings, err := s.embeddingService.Embed(ctx, documents, "")
	if err != nil {
		s.logger.Error("TOOL ERROR: insert_docs_vector_store failed (embed)", zap.Error(err))
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Insert documents
	insertQuery := fmt.Sprintf(
		"INSERT INTO `%s`.`%s` (document, embedding, metadata) VALUES (?, VEC_FromText(?), ?)",
		databaseName, vectorStoreName,
	)

	inserted := 0
	var errors []string

	for i, doc := range documents {
		embedding := embeddings[i]
		metadata := metadataList[i]

		// Convert embedding to JSON string for VEC_FromText
		embJSON, err := json.Marshal(embedding)
		if err != nil {
			errors = append(errors, fmt.Sprintf("doc %d: failed to marshal embedding: %v", i, err))
			continue
		}

		// Convert metadata to JSON
		metaJSON, err := json.Marshal(metadata)
		if err != nil {
			errors = append(errors, fmt.Sprintf("doc %d: failed to marshal metadata: %v", i, err))
			continue
		}

		err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
			// Switch to target database
			if _, err := conn.ExecContext(ctx, "USE `"+databaseName+"`"); err != nil {
				return err
			}
			_, err := conn.ExecContext(ctx, insertQuery, doc, string(embJSON), string(metaJSON))
			return err
		})

		if err != nil {
			errors = append(errors, fmt.Sprintf("doc %d: %v", i, err))
		} else {
			inserted++
		}
	}

	s.logger.Info("TOOL END: insert_docs_vector_store completed",
		zap.Int("inserted", inserted),
		zap.Int("errors", len(errors)))

	result := map[string]interface{}{
		"status":   "success",
		"inserted": inserted,
	}
	if inserted < len(documents) {
		result["status"] = "partial"
		result["errors"] = errors
	}

	return result, nil
}

// searchVectorStore searches a vector store for similar documents
func (s *Server) searchVectorStore(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	userQuery, ok := args["user_query"].(string)
	if !ok {
		return nil, fmt.Errorf("user_query is required")
	}
	databaseName, ok := args["database_name"].(string)
	if !ok {
		return nil, fmt.Errorf("database_name is required")
	}
	vectorStoreName, ok := args["vector_store_name"].(string)
	if !ok {
		return nil, fmt.Errorf("vector_store_name is required")
	}

	if v := validateIdentifier(databaseName); !v.Valid {
		return nil, fmt.Errorf("invalid database_name: %s", v.Error)
	}
	if v := validateIdentifier(vectorStoreName); !v.Valid {
		return nil, fmt.Errorf("invalid vector_store_name: %s", v.Error)
	}
	if userQuery == "" {
		return nil, fmt.Errorf("user_query must not be empty")
	}

	// Get k parameter (default 7)
	k := 7
	if kRaw, ok := args["k"].(float64); ok {
		k = int(kRaw)
	} else if kRaw, ok := args["k"].(int); ok {
		k = kRaw
	}
	if k <= 0 {
		k = 7
	}

	s.logger.Info("TOOL START: search_vector_store called",
		zap.String("database", databaseName),
		zap.String("store", vectorStoreName),
		zap.Int("k", k),
	)

	// Generate embedding for query
	embedding, err := s.embeddingService.Embed(ctx, []string{userQuery}, "")
	if err != nil {
		s.logger.Error("TOOL ERROR: search_vector_store failed (embed)", zap.Error(err))
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Convert embedding to JSON for VEC_FromText
	embJSON, err := json.Marshal(embedding[0])
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding: %w", err)
	}

	// Execute search query
	searchQuery := fmt.Sprintf(`
		SELECT
			document,
			metadata,
			VEC_DISTANCE_COSINE(embedding, VEC_FromText(?)) AS distance
		FROM %s.%s
		ORDER BY distance ASC
		LIMIT ?
	`, "`"+databaseName+"`", "`"+vectorStoreName+"`")

	type resultRow struct {
		Document string `db:"document"`
		Metadata string `db:"metadata"`
		Distance float64 `db:"distance"`
	}

	var rows []resultRow
	err = s.pool.WithContext(ctx, func(conn *sqlx.Conn) error {
		// Switch to target database
		if _, err := conn.ExecContext(ctx, "USE `"+databaseName+"`"); err != nil {
			return err
		}
		return conn.SelectContext(ctx, &rows, searchQuery, string(embJSON), k)
	})

	if err != nil {
		s.logger.Error("TOOL ERROR: search_vector_store failed", zap.Error(err))
		return nil, fmt.Errorf("search query failed: %w", err)
	}

	// Parse metadata JSON
	results := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		var metadata map[string]interface{}
		_ = json.Unmarshal([]byte(row.Metadata), &metadata)
		results[i] = map[string]interface{}{
			"document": row.Document,
			"metadata": metadata,
			"distance": row.Distance,
		}
	}

	s.logger.Info("TOOL END: search_vector_store completed", zap.Int("results", len(results)))
	return results, nil
}
