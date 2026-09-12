package service

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func databasePortByType(dbType string, value int) int {
	if value > 0 {
		return value
	}
	switch normalizeDatabaseType(dbType) {
	case "postgresql":
		return 5432
	case "mongodb":
		return 27017
	case "redis":
		return 6379
	default:
		return 3306
	}
}

func databaseCharsetByType(dbType, value string) string {
	if normalizeDatabaseType(dbType) != "mysql" {
		return strings.TrimSpace(value)
	}
	return databaseCharset(value)
}

func databaseRequiresUsername(dbType string) bool {
	switch normalizeDatabaseType(dbType) {
	case "mongodb", "redis":
		return false
	default:
		return true
	}
}

func databaseCapabilities(dbType string) map[string]bool {
	switch normalizeDatabaseType(dbType) {
	case "postgresql":
		return map[string]bool{"sql": true, "schema": true, "tableData": true, "resourceData": false, "rowEdit": true, "transfer": false, "import": true, "export": false, "backup": true}
	case "mongodb":
		return map[string]bool{"sql": false, "schema": true, "tableData": false, "resourceData": true, "rowEdit": false, "transfer": false, "backup": false}
	case "redis":
		return map[string]bool{"sql": false, "schema": true, "tableData": false, "resourceData": true, "rowEdit": false, "keyEdit": true, "transfer": false, "backup": false}
	default:
		return map[string]bool{"sql": true, "schema": true, "tableData": true, "resourceData": false, "rowEdit": true, "transfer": true, "backup": true}
	}
}

func databaseMode(dbType string) string {
	switch normalizeDatabaseType(dbType) {
	case "mongodb":
		return "document"
	case "redis":
		return "keyvalue"
	default:
		return "sql"
	}
}

// ensureMySQLFeature prevents MySQL-specific endpoints from being applied to
// other database engines through a direct API call.
func ensureMySQLFeature(item *model.AssetDatabase) error {
	if normalizeDatabaseType(item.DBType) != "mysql" {
		return fmt.Errorf("%s does not support this operation; use capabilities supported by this database type in the workbench", databaseTypeDisplayName(item.DBType))
	}
	return nil
}

// ensureRelationalImportFeature limits table import to engines with a tabular
// schema. MongoDB and Redis use their own resource models and are excluded.
func ensureRelationalImportFeature(item *model.AssetDatabase) error {
	switch normalizeDatabaseType(item.DBType) {
	case "mysql", "postgresql":
		return nil
	default:
		return fmt.Errorf("%s does not support table-data import", databaseTypeDisplayName(item.DBType))
	}
}

func importTableName(item *model.AssetDatabase, schema, table string) string {
	if normalizeDatabaseType(item.DBType) == "postgresql" {
		return postgresTableName(schema, table)
	}
	return quoteIdentifier(schema) + "." + quoteIdentifier(table)
}

func importJoinIdentifiers(item *model.AssetDatabase, columns []string) string {
	if normalizeDatabaseType(item.DBType) == "postgresql" {
		return postgresJoinIdentifiers(columns)
	}
	return joinIdentifiers(columns)
}

func importPlaceholders(item *model.AssetDatabase, count int) string {
	items := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		if normalizeDatabaseType(item.DBType) == "postgresql" {
			items = append(items, fmt.Sprintf("$%d", index))
		} else {
			items = append(items, "?")
		}
	}
	return strings.Join(items, ",")
}

func (s *Service) getImportTableColumns(item *model.AssetDatabase, db *sql.DB, schema, table string) ([]databaseTableColumn, error) {
	if normalizeDatabaseType(item.DBType) == "postgresql" {
		return postgresTableColumns(db, schema, table)
	}
	return s.getTableColumns(db, schema, table)
}

func (s *Service) createPostgresImportTable(sourceDB, targetDB *sql.DB, sourceSchema, sourceTable, targetSchema, targetTable string) error {
	rows, err := sourceDB.Query(`
		SELECT a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod),
			a.attnotnull,
			pg_get_expr(ad.adbin, ad.adrelid)
		FROM pg_catalog.pg_attribute AS a
		JOIN pg_catalog.pg_class AS c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		LEFT JOIN pg_catalog.pg_attrdef AS ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
		WHERE n.nspname = $1 AND c.relname = $2
			AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, sourceSchema, sourceTable)
	if err != nil {
		return err
	}
	defer rows.Close()

	definitions := make([]string, 0)
	for rows.Next() {
		var name, dataType string
		var notNull bool
		var defaultExpr sql.NullString
		if err := rows.Scan(&name, &dataType, &notNull, &defaultExpr); err != nil {
			return err
		}
		definition := postgresQuoteIdentifier(name) + " " + dataType
		// Sequence defaults are tied to the source table. Imported data carries
		// explicit values, so omit them rather than retaining a broken reference.
		if defaultExpr.Valid && strings.TrimSpace(defaultExpr.String) != "" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(defaultExpr.String)), "nextval(") {
			definition += " DEFAULT " + defaultExpr.String
		}
		if notNull {
			definition += " NOT NULL"
		}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(definitions) == 0 {
		return fmt.Errorf("source table does not exist or has no importable columns")
	}

	primaryRows, err := sourceDB.Query(`
		SELECT a.attname
		FROM pg_catalog.pg_constraint AS con
		JOIN pg_catalog.pg_class AS c ON c.oid = con.conrelid
		JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_attribute AS a ON a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
		WHERE con.contype = 'p' AND n.nspname = $1 AND c.relname = $2
		ORDER BY array_position(con.conkey, a.attnum)`, sourceSchema, sourceTable)
	if err != nil {
		return err
	}
	primaryKeys := make([]string, 0)
	for primaryRows.Next() {
		var name string
		if err := primaryRows.Scan(&name); err != nil {
			primaryRows.Close()
			return err
		}
		primaryKeys = append(primaryKeys, postgresQuoteIdentifier(name))
	}
	if err := primaryRows.Err(); err != nil {
		primaryRows.Close()
		return err
	}
	primaryRows.Close()
	if len(primaryKeys) > 0 {
		definitions = append(definitions, "PRIMARY KEY ("+strings.Join(primaryKeys, ", ")+")")
	}

	createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", postgresTableName(targetSchema, targetTable), strings.Join(definitions, ", "))
	_, err = targetDB.Exec(createSQL)
	return err
}

func databaseTypeDisplayName(dbType string) string {
	switch normalizeDatabaseType(dbType) {
	case "postgresql":
		return "PostgreSQL"
	case "mongodb":
		return "MongoDB"
	case "redis":
		return "Redis"
	default:
		return "MySQL"
	}
}

func (s *Service) resolveAssetDatabaseTarget(item model.AssetDatabase) (model.AssetDatabase, func(), error) {
	target := item
	cleanup := func() {}
	if normalizeConnectionMode(item.ConnectionMode) != "gateway" || item.GatewayID == nil || *item.GatewayID == 0 {
		return target, cleanup, nil
	}
	address, tunnelCleanup, err := s.startGatewayTunnel(*item.GatewayID, net.JoinHostPort(strings.TrimSpace(item.Host), strconv.Itoa(databasePortByType(item.DBType, item.Port))))
	if err != nil {
		return target, cleanup, err
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		tunnelCleanup()
		return target, cleanup, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		tunnelCleanup()
		return target, cleanup, err
	}
	target.Host, target.Port, cleanup = host, port, tunnelCleanup
	return target, cleanup, nil
}

func postgresDSN(item model.AssetDatabase) string {
	database := strings.TrimSpace(item.DBName)
	if database == "" {
		database = "postgres"
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=5", item.Host, databasePortByType("postgresql", item.Port), item.Username, item.Password, database)
}

func (s *Service) openAssetPostgresDatabase(item model.AssetDatabase) (*sql.DB, func(), error) {
	target, cleanup, err := s.resolveAssetDatabaseTarget(item)
	if err != nil {
		return nil, cleanup, err
	}
	db, err := sql.Open("pgx", postgresDSN(target))
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return db, cleanup, nil
}

func (s *Service) inspectAssetDatabase(item model.AssetDatabase) (string, error) {
	switch normalizeDatabaseType(item.DBType) {
	case "postgresql":
		db, cleanup, err := s.openAssetPostgresDatabase(item)
		if err != nil {
			return "", err
		}
		defer cleanup()
		defer db.Close()
		if err := db.Ping(); err != nil {
			return "", err
		}
		var version string
		err = db.QueryRow("SHOW server_version").Scan(&version)
		return version, err
	case "mongodb":
		target, cleanup, err := s.resolveAssetDatabaseTarget(item)
		if err != nil {
			return "", err
		}
		defer cleanup()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%d", target.Host, databasePortByType("mongodb", target.Port))).SetAuth(options.Credential{Username: target.Username, Password: target.Password}))
		if err != nil {
			return "", err
		}
		defer client.Disconnect(context.Background())
		var info struct {
			Version string `bson:"version"`
		}
		err = client.Database("admin").RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&info)
		return info.Version, err
	case "redis":
		target, cleanup, err := s.resolveAssetDatabaseTarget(item)
		if err != nil {
			return "", err
		}
		defer cleanup()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		dbIndex, _ := strconv.Atoi(strings.TrimSpace(target.DBName))
		client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(target.Host, strconv.Itoa(databasePortByType("redis", target.Port))), Username: target.Username, Password: target.Password, DB: dbIndex})
		defer client.Close()
		if err := client.Ping(ctx).Err(); err != nil {
			return "", err
		}
		info, err := client.Info(ctx, "server").Result()
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(info, "\n") {
			if strings.HasPrefix(line, "redis_version:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "redis_version:")), nil
			}
		}
		return "Redis", nil
	default:
		return s.inspectAssetMySQLDatabase(item)
	}
}

func (s *Service) getNonMySQLSchemaTree(item *model.AssetDatabase) (map[string]any, error) {
	switch normalizeDatabaseType(item.DBType) {
	case "postgresql":
		db, cleanup, err := s.openAssetPostgresDatabase(*item)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		defer db.Close()
		rows, err := db.Query(`
			SELECT schema_name
			FROM information_schema.schemata
			WHERE schema_name NOT IN ('pg_catalog', 'information_schema')
			  AND schema_name NOT LIKE 'pg_%'
			ORDER BY schema_name
		`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		schemas := make([]map[string]any, 0)
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			tableRows, err := db.Query(`SELECT table_name, table_type FROM information_schema.tables WHERE table_schema = $1 ORDER BY table_name`, name)
			if err != nil {
				return nil, err
			}
			type tableMeta struct {
				name      string
				tableType string
			}
			metadata := make([]tableMeta, 0)
			for tableRows.Next() {
				var tableName, tableType string
				if err := tableRows.Scan(&tableName, &tableType); err != nil {
					tableRows.Close()
					return nil, err
				}
				metadata = append(metadata, tableMeta{name: tableName, tableType: tableType})
			}
			if err := tableRows.Err(); err != nil {
				tableRows.Close()
				return nil, err
			}
			tableRows.Close()

			tables := make([]map[string]any, 0, len(metadata))
			for _, table := range metadata {
				var rowCount any
				if strings.EqualFold(table.tableType, "BASE TABLE") {
					var count int64
					countQuery := "SELECT COUNT(*) FROM " + postgresQuoteIdentifier(name) + "." + postgresQuoteIdentifier(table.name)
					if err := db.QueryRow(countQuery).Scan(&count); err == nil {
						rowCount = count
					}
				}
				tables = append(tables, map[string]any{
					"name":     table.name,
					"type":     table.tableType,
					"rows":     rowCount,
					"schema":   name,
					"fullName": name + "." + table.name,
				})
			}
			schemas = append(schemas, map[string]any{"name": name, "tableCount": len(tables), "tables": tables, "isCurrent": name == defaultSchema(item, "")})
		}
		return map[string]any{"schemas": schemas, "defaultSchema": defaultSchema(item, "")}, nil
	case "mongodb":
		target, cleanup, err := s.resolveAssetDatabaseTarget(*item)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%d", target.Host, databasePortByType("mongodb", target.Port))).SetAuth(options.Credential{Username: target.Username, Password: target.Password}))
		if err != nil {
			return nil, err
		}
		defer client.Disconnect(context.Background())
		names, err := client.ListDatabaseNames(ctx, bson.D{})
		if err != nil {
			return nil, err
		}
		schemas := make([]map[string]any, 0, len(names))
		for _, name := range names {
			collections, _ := client.Database(name).ListCollectionNames(ctx, bson.D{})
			tables := make([]map[string]any, 0, len(collections))
			for _, collection := range collections {
				tables = append(tables, map[string]any{"name": collection, "type": "collection", "rows": 0, "schema": name, "fullName": name + "." + collection})
			}
			schemas = append(schemas, map[string]any{"name": name, "tableCount": len(tables), "tables": tables, "isCurrent": name == strings.TrimSpace(item.DBName)})
		}
		return map[string]any{"schemas": schemas, "defaultSchema": defaultSchema(item, "")}, nil
	case "redis":
		target, cleanup, err := s.resolveAssetDatabaseTarget(*item)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		dbIndex, _ := strconv.Atoi(strings.TrimSpace(target.DBName))
		client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(target.Host, strconv.Itoa(databasePortByType("redis", target.Port))), Username: target.Username, Password: target.Password, DB: dbIndex})
		defer client.Close()
		if err := client.Ping(ctx).Err(); err != nil {
			return nil, err
		}
		keys, _, err := client.Scan(ctx, 0, "*", 100).Result()
		if err != nil {
			return nil, err
		}
		tables := make([]map[string]any, 0, len(keys))
		for _, key := range keys {
			keyType, _ := client.Type(ctx, key).Result()
			tables = append(tables, map[string]any{"name": key, "type": keyType, "rows": 0, "schema": fmt.Sprintf("db%d", dbIndex), "fullName": key})
		}
		return map[string]any{"schemas": []map[string]any{{"name": fmt.Sprintf("db%d", dbIndex), "tableCount": len(tables), "tables": tables, "isCurrent": true}}, "defaultSchema": fmt.Sprintf("db%d", dbIndex)}, nil
	default:
		return nil, fmt.Errorf("unsupported database type")
	}
}

// GetDatabaseResourceData provides a read-only browser for database engines
// whose resources are not relational MySQL tables.
func (s *Service) GetDatabaseResourceData(payload DBMSTableDataQueryPayload) (map[string]any, error) {
	if payload.DatabaseID == 0 || strings.TrimSpace(payload.Table) == "" {
		return nil, fmt.Errorf("please select a resource first")
	}
	if payload.PageNum < 1 {
		payload.PageNum = 1
	}
	if payload.PageSize < 1 || payload.PageSize > 200 {
		payload.PageSize = 25
	}
	item, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	switch normalizeDatabaseType(item.DBType) {
	case "postgresql":
		return s.getPostgresResourceData(item, payload)
	case "mongodb":
		return s.getMongoResourceData(item, payload)
	case "redis":
		return s.getRedisResourceData(item, payload)
	default:
		return nil, fmt.Errorf("resource browser is only available for PostgreSQL, MongoDB, and Redis")
	}
}
