package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Service) getMongoResourceData(item *model.AssetDatabase, payload DBMSTableDataQueryPayload) (map[string]any, error) {
	target, cleanup, err := s.resolveAssetDatabaseTarget(*item)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	clientOptions := options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%d", target.Host, databasePortByType("mongodb", target.Port)))
	if strings.TrimSpace(target.Username) != "" || target.Password != "" {
		clientOptions.SetAuth(options.Credential{Username: target.Username, Password: target.Password})
	}
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(context.Background())
	schema := strings.TrimSpace(payload.Schema)
	if schema == "" {
		schema = strings.TrimSpace(target.DBName)
	}
	collection := client.Database(schema).Collection(payload.Table)
	filter := bson.M{}
	if payload.FilterKey != "" && strings.TrimSpace(payload.FilterText) != "" {
		filter[payload.FilterKey] = bson.M{"$regex": strings.TrimSpace(payload.FilterText), "$options": "i"}
	}
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	cursor, err := collection.Find(ctx, filter, options.Find().SetSkip(int64((payload.PageNum-1)*payload.PageSize)).SetLimit(int64(payload.PageSize)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	rows := make([]map[string]any, 0)
	keys := map[string]bool{}
	for cursor.Next(ctx) {
		var document bson.M
		if err := cursor.Decode(&document); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for key, value := range document {
			row[key] = value
			keys[key] = true
		}
		rows = append(rows, row)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	indexesCursor, err := collection.Indexes().List(ctx)
	indexes := make([]map[string]any, 0)
	if err == nil {
		defer indexesCursor.Close(ctx)
		for indexesCursor.Next(ctx) {
			var index bson.M
			if indexesCursor.Decode(&index) == nil {
				indexes = append(indexes, index)
			}
		}
	}
	columnNames := make([]string, 0, len(keys))
	for key := range keys {
		columnNames = append(columnNames, key)
	}
	sort.Strings(columnNames)
	columns := make([]databaseTableColumn, 0, len(columnNames))
	for _, key := range columnNames {
		columns = append(columns, databaseTableColumn{Name: key, DataType: "document"})
	}
	return map[string]any{"schema": schema, "table": payload.Table, "columns": columns, "rows": rows, "total": total, "pageNum": payload.PageNum, "pageSize": payload.PageSize, "indexes": indexes, "readOnly": true, "resourceType": "collection"}, nil
}

func (s *Service) getRedisResourceData(item *model.AssetDatabase, payload DBMSTableDataQueryPayload) (map[string]any, error) {
	target, cleanup, err := s.resolveAssetDatabaseTarget(*item)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	dbIndex, _ := strconv.Atoi(strings.TrimSpace(target.DBName))
	client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(target.Host, strconv.Itoa(databasePortByType("redis", target.Port))), Username: target.Username, Password: target.Password, DB: dbIndex})
	defer client.Close()
	key := payload.Table
	keyType, err := client.Type(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	value := ""
	switch keyType {
	case "string":
		value, _ = client.Get(ctx, key).Result()
	case "list":
		items, _ := client.LRange(ctx, key, 0, 199).Result()
		encoded, _ := json.Marshal(items)
		value = string(encoded)
	case "set":
		items, _ := client.SMembers(ctx, key).Result()
		encoded, _ := json.Marshal(items)
		value = string(encoded)
	case "zset":
		items, _ := client.ZRangeWithScores(ctx, key, 0, 199).Result()
		encoded, _ := json.Marshal(items)
		value = string(encoded)
	case "hash":
		items, _ := client.HGetAll(ctx, key).Result()
		encoded, _ := json.Marshal(items)
		value = string(encoded)
	case "stream":
		count, _ := client.XLen(ctx, key).Result()
		value = fmt.Sprintf("stream (%d entries)", count)
	default:
		value = keyType
	}
	ttlSeconds := int64(ttl / time.Second)
	ttlText := fmt.Sprintf("%d seconds", ttlSeconds)
	if ttl == -1*time.Nanosecond {
		ttlSeconds, ttlText = -1, "Never expires"
	} else if ttl == -2*time.Nanosecond {
		ttlSeconds, ttlText = -2, "Expired"
	}
	return map[string]any{
		"schema": payload.Schema, "table": key, "columns": []databaseTableColumn{{Name: "key"}, {Name: "type"}, {Name: "ttl"}, {Name: "value"}},
		"rows": []map[string]any{{"key": key, "type": keyType, "ttl": ttlText, "ttlSeconds": ttlSeconds, "value": value}}, "total": 1, "pageNum": 1, "pageSize": 1,
		"readOnly": normalizeDatabaseAccessMode(item.AccessMode) == "readonly", "resourceType": "key",
	}, nil
}

type RedisCommandPayload struct {
	DatabaseID  uint   `json:"databaseId"`
	CommandText string `json:"commandText"`
	Confirmed   bool   `json:"confirmed"`
	Operator    string `json:"-"`
	ClientIP    string `json:"-"`
}

type RedisCommandAnalysis struct {
	Command        string `json:"command"`
	Arguments      int    `json:"arguments"`
	WriteOperation bool   `json:"writeOperation"`
	RiskLevel      string `json:"riskLevel"`
	Reason         string `json:"reason"`
	AccessMode     string `json:"accessMode"`
}

func splitRedisCommand(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("enter a Redis command")
	}
	args := make([]string, 0)
	var current strings.Builder
	quote := rune(0)
	escaped := false
	for _, char := range input {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				current.WriteRune(char)
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == ' ' || char == '\t' || char == '\n' || char == '\r' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(char)
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("Redis command contains an unclosed quote")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("enter a Redis command")
	}
	return args, nil
}

func isRedisReadCommand(command string) bool {
	switch strings.ToUpper(command) {
	case "GET", "MGET", "EXISTS", "TTL", "PTTL", "TYPE", "KEYS", "SCAN", "HSCAN", "SSCAN", "ZSCAN", "LRANGE", "LLEN", "LINDEX", "SMEMBERS", "SCARD", "SISMEMBER", "ZRANGE", "ZRANGEBYSCORE", "ZSCORE", "ZCARD", "HGET", "HMGET", "HGETALL", "HLEN", "HEXISTS", "DBSIZE", "INFO", "MEMORY", "PING", "TIME":
		return true
	default:
		return false
	}
}

func isBlockedRedisCommand(command string) bool {
	switch strings.ToUpper(command) {
	case "FLUSHALL", "FLUSHDB", "SHUTDOWN", "DEBUG", "MODULE", "ACL", "REPLICAOF", "SLAVEOF", "CONFIG", "MIGRATE":
		return true
	default:
		return false
	}
}

func (s *Service) AnalyzeRedisCommand(payload RedisCommandPayload) (*RedisCommandAnalysis, error) {
	item, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	if normalizeDatabaseType(item.DBType) != "redis" {
		return nil, fmt.Errorf("selected database is not Redis")
	}
	args, err := splitRedisCommand(payload.CommandText)
	if err != nil {
		return nil, err
	}
	command := strings.ToUpper(args[0])
	if isBlockedRedisCommand(command) {
		return nil, fmt.Errorf("Redis command console does not allow %s", command)
	}
	writeOperation := !isRedisReadCommand(command)
	reason := "read-only command; execution is allowed"
	riskLevel := "low"
	if writeOperation {
		reason = "write command; explicit confirmation is required"
		riskLevel = "medium"
	}
	if writeOperation && normalizeDatabaseAccessMode(item.AccessMode) == "readonly" {
		reason = "current Redis connection is read-only"
		riskLevel = "high"
	}
	return &RedisCommandAnalysis{Command: command, Arguments: len(args) - 1, WriteOperation: writeOperation, RiskLevel: riskLevel, Reason: reason, AccessMode: normalizeDatabaseAccessMode(item.AccessMode)}, nil
}

func redisReplyText(value any) string {
	if value == nil {
		return "(nil)"
	}
	switch v := value.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, redisReplyText(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []string:
		return "[" + strings.Join(v, ", ") + "]"
	default:
		return fmt.Sprint(value)
	}
}

func (s *Service) ExecuteRedisCommand(payload RedisCommandPayload) (map[string]any, error) {
	analysis, err := s.AnalyzeRedisCommand(payload)
	if err != nil {
		return nil, err
	}
	if analysis.WriteOperation && analysis.AccessMode == "readonly" {
		return nil, fmt.Errorf("current Redis connection is read-only; write commands are not allowed")
	}
	if analysis.WriteOperation && !payload.Confirmed {
		return nil, fmt.Errorf("write command requires confirmation before execution")
	}
	args, _ := splitRedisCommand(payload.CommandText)
	item, err := s.getAssetDatabase(payload.DatabaseID)
	if err != nil {
		return nil, err
	}
	target, cleanup, err := s.resolveAssetDatabaseTarget(*item)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dbIndex, _ := strconv.Atoi(strings.TrimSpace(target.DBName))
	client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(target.Host, strconv.Itoa(databasePortByType("redis", target.Port))), Username: target.Username, Password: target.Password, DB: dbIndex})
	defer client.Close()
	start := time.Now()
	commandArgs := make([]interface{}, 0, len(args))
	for _, arg := range args {
		commandArgs = append(commandArgs, arg)
	}
	reply, execErr := client.Do(ctx, commandArgs...).Result()
	durationMs := time.Since(start).Milliseconds()
	history := model.DatabaseSQLHistory{DatabaseID: item.ID, DatabaseName: item.Name, SchemaName: fmt.Sprintf("db%d", dbIndex), SQLType: "REDIS " + analysis.Command, SQLText: strings.TrimSpace(payload.CommandText), ExecutionID: newDBMSExecutionID(), Operator: strings.TrimSpace(payload.Operator), ClientIP: strings.TrimSpace(payload.ClientIP), Environment: item.Env, AccessMode: normalizeDatabaseAccessMode(item.AccessMode), DurationMs: durationMs}
	if execErr != nil {
		history.Status = 2
		history.ErrorMessage = execErr.Error()
		s.db.Create(&history)
		return nil, execErr
	}
	history.Status = 1
	history.RowsAffected = 1
	s.db.Create(&history)
	return map[string]any{"command": analysis.Command, "columns": []string{"command", "result"}, "rows": []map[string]any{{"command": strings.TrimSpace(payload.CommandText), "result": redisReplyText(reply)}}, "rowsAffected": 1, "durationMs": durationMs, "executionId": history.ExecutionID, "analysis": analysis}, nil
}
