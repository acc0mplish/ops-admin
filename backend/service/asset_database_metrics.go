package service

import (
	"context"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"ops-admin/backend/model"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetAssetDatabaseMetrics collects one native sample when the dashboard is
// enabled, then reads the locally retained samples for the requested range.
// No Prometheus/VictoriaMetrics datasource is required.
func (s *Service) GetAssetDatabaseMetrics(id uint, rangeKey, startValue, endValue string) (map[string]any, error) {
	item, err := s.getAssetDatabase(id)
	if err != nil { return nil, err }
	startAt, endAt, _, normalizedRange, err := resolveAssetHostMetricRange(rangeKey, startValue, endValue)
	if err != nil { return nil, err }
	response := map[string]any{"enabled": item.MonitorEnabled, "range": map[string]any{"key": normalizedRange, "startAt": startAt, "endAt": endAt}, "metrics": emptyDatabaseMetricSeries()}
	if !item.MonitorEnabled { response["status"] = "disabled"; return response, nil }
	if sample, collectErr := s.collectAssetDatabaseMetrics(item); collectErr == nil {
		if err := s.db.Create(&sample).Error; err != nil { response["collectError"] = err.Error() }
	} else { response["collectError"] = collectErr.Error() }
	endAt = time.Now()
	var snapshots []model.AssetDatabaseMetricSnapshot
	if err := s.db.Where("database_id = ? AND created_at >= ? AND created_at <= ?", id, startAt, endAt).Order("created_at ASC").Find(&snapshots).Error; err != nil { return nil, err }
	response["metrics"] = databaseMetricSeries(snapshots)
	if len(snapshots) == 0 { response["status"] = "unavailable" } else { response["status"] = "available" }
	return response, nil
}

func emptyDatabaseMetricSeries() map[string]any {
	metrics := map[string]any{}
	for _, key := range databaseMetricKeys() { metrics[key] = map[string]any{"latest": nil, "points": []map[string]any{}} }
	return metrics
}

func databaseMetricKeys() []string { return []string{"qps", "connections", "threads", "bytesIn", "bytesOut", "slowQueries", "uptime", "cacheHitRate"} }

func databaseMetricSeries(snapshots []model.AssetDatabaseMetricSnapshot) map[string]any {
	metrics := emptyDatabaseMetricSeries()
	for _, key := range databaseMetricKeys() {
		points := make([]map[string]any, 0, len(snapshots))
		for _, snapshot := range snapshots { if value, ok := snapshot.Metrics[key]; ok && !math.IsNaN(value) && !math.IsInf(value, 0) { points = append(points, map[string]any{"timestamp": snapshot.CreatedAt.UnixMilli(), "value": value}) } }
		var latest any
		if len(points) > 0 { latest = points[len(points)-1]["value"] }
		metrics[key] = map[string]any{"latest": latest, "points": points}
	}
	return metrics
}

func (s *Service) collectAssetDatabaseMetrics(item *model.AssetDatabase) (model.AssetDatabaseMetricSnapshot, error) {
	current, err := s.readAssetDatabaseMetricCounters(*item)
	if err != nil { return model.AssetDatabaseMetricSnapshot{}, err }
	var previous model.AssetDatabaseMetricSnapshot
	_ = s.db.Where("database_id = ?", item.ID).Order("created_at DESC").First(&previous).Error
	metrics := deriveDatabaseMetrics(current, previous.Metrics, time.Since(previous.CreatedAt))
	for _, key := range []string{"counterQueries", "counterBytesIn", "counterBytesOut", "counterSlowQueries"} { metrics[key] = current[key] }
	return model.AssetDatabaseMetricSnapshot{DatabaseID: item.ID, Metrics: metrics}, nil
}

func deriveDatabaseMetrics(current, previous map[string]float64, elapsed time.Duration) map[string]float64 {
	metrics := map[string]float64{}
	for _, key := range []string{"connections", "threads", "uptime", "cacheHitRate"} { metrics[key] = current[key] }
	seconds := elapsed.Seconds()
	if previous == nil || seconds <= 0 || seconds > 24*time.Hour.Seconds() { metrics["qps"], metrics["bytesIn"], metrics["bytesOut"], metrics["slowQueries"] = 0, 0, 0, 0; return metrics }
	metrics["qps"] = counterRate(current["counterQueries"], previous["counterQueries"], seconds)
	metrics["bytesIn"] = counterRate(current["counterBytesIn"], previous["counterBytesIn"], seconds)
	metrics["bytesOut"] = counterRate(current["counterBytesOut"], previous["counterBytesOut"], seconds)
	metrics["slowQueries"] = counterRate(current["counterSlowQueries"], previous["counterSlowQueries"], seconds)
	return metrics
}

func counterRate(current, previous, seconds float64) float64 { if current < previous || seconds <= 0 { return 0 }; return (current - previous) / seconds }

func (s *Service) readAssetDatabaseMetricCounters(item model.AssetDatabase) (map[string]float64, error) {
	switch normalizeDatabaseType(item.DBType) {
	case "mysql": return s.readMySQLDatabaseMetrics(item)
	case "postgresql": return s.readPostgresDatabaseMetrics(item)
	case "mongodb": return s.readMongoDatabaseMetrics(item)
	case "redis": return s.readRedisDatabaseMetrics(item)
	default: return nil, fmt.Errorf("database type %s is not supported by the monitoring dashboard", item.DBType)
	}
}

func metricValue(values map[string]float64, key string) float64 { return values[strings.ToLower(key)] }

func (s *Service) readMySQLDatabaseMetrics(item model.AssetDatabase) (map[string]float64, error) {
	db, cleanup, err := s.openAssetMySQLDatabase(item, ""); if err != nil { return nil, err }; defer cleanup()
	rows, err := db.Query(`SHOW GLOBAL STATUS WHERE Variable_name IN ('Queries','Threads_connected','Threads_running','Bytes_received','Bytes_sent','Slow_queries','Uptime','Innodb_buffer_pool_reads','Innodb_buffer_pool_read_requests')`); if err != nil { return nil, err }; defer rows.Close()
	values := map[string]float64{}
	for rows.Next() { var name, value string; if err := rows.Scan(&name, &value); err != nil { return nil, err }; values[strings.ToLower(name)], _ = strconv.ParseFloat(value, 64) }
	reads, requests := metricValue(values, "Innodb_buffer_pool_reads"), metricValue(values, "Innodb_buffer_pool_read_requests")
	hitRate := 0.0; if requests > 0 { hitRate = math.Max(0, 100*(1-reads/requests)) }
	return map[string]float64{"counterQueries": metricValue(values, "Queries"), "connections": metricValue(values, "Threads_connected"), "threads": metricValue(values, "Threads_running"), "counterBytesIn": metricValue(values, "Bytes_received"), "counterBytesOut": metricValue(values, "Bytes_sent"), "counterSlowQueries": metricValue(values, "Slow_queries"), "uptime": metricValue(values, "Uptime"), "cacheHitRate": hitRate}, rows.Err()
}

func (s *Service) readPostgresDatabaseMetrics(item model.AssetDatabase) (map[string]float64, error) {
	db, cleanup, err := s.openAssetPostgresDatabase(item); if err != nil { return nil, err }; defer cleanup()
	row := db.QueryRow(`SELECT xact_commit + xact_rollback, numbackends, tup_returned + tup_fetched + tup_inserted + tup_updated + tup_deleted, blks_hit, blks_read, EXTRACT(EPOCH FROM (now() - pg_postmaster_start_time())) FROM pg_stat_database WHERE datname = current_database()`)
	var queries, connections, operations, hit, read, uptime float64
	if err := row.Scan(&queries, &connections, &operations, &hit, &read, &uptime); err != nil { return nil, err }
	hitRate := 0.0; if hit+read > 0 { hitRate = 100 * hit / (hit + read) }
	return map[string]float64{"counterQueries": queries + operations, "connections": connections, "threads": connections, "counterBytesIn": 0, "counterBytesOut": 0, "counterSlowQueries": 0, "uptime": uptime, "cacheHitRate": hitRate}, nil
}

func (s *Service) readMongoDatabaseMetrics(item model.AssetDatabase) (map[string]float64, error) {
	target, cleanup, err := s.resolveAssetDatabaseTarget(item); if err != nil { return nil, err }; defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second); defer cancel()
	uri := fmt.Sprintf("mongodb://%s", net.JoinHostPort(target.Host, strconv.Itoa(databasePortByType("mongodb", target.Port))))
	client, err := mongoConnectWithAuth(ctx, uri, target.Username, target.Password); if err != nil { return nil, err }; defer client.Disconnect(context.Background())
	var status bson.M; if err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}).Decode(&status); err != nil { return nil, err }
	get := func(path ...string) float64 { var value any = status; for _, key := range path { data, ok := value.(bson.M); if !ok { return 0 }; value = data[key] }; switch v := value.(type) { case int32: return float64(v); case int64: return float64(v); case float64: return v; default: return 0 } }
	queries := get("opcounters", "query") + get("opcounters", "command") + get("opcounters", "insert") + get("opcounters", "update") + get("opcounters", "delete")
	hitRate := 0.0; hits, misses := get("wiredTiger", "cache", "pages read into cache"), get("wiredTiger", "cache", "pages requested from the cache"); if hits+misses > 0 { hitRate = 100 * misses / (hits + misses) }
	return map[string]float64{"counterQueries": queries, "connections": get("connections", "current"), "threads": get("globalLock", "activeClients", "total"), "counterBytesIn": get("network", "bytesIn"), "counterBytesOut": get("network", "bytesOut"), "counterSlowQueries": 0, "uptime": get("uptime"), "cacheHitRate": hitRate}, nil
}

func mongoConnectWithAuth(ctx context.Context, uri, username, password string) (*mongo.Client, error) { clientOptions := options.Client().ApplyURI(uri); if username != "" || password != "" { clientOptions.SetAuth(options.Credential{Username: username, Password: password}) }; return mongo.Connect(ctx, clientOptions) }

func (s *Service) readRedisDatabaseMetrics(item model.AssetDatabase) (map[string]float64, error) {
	target, cleanup, err := s.resolveAssetDatabaseTarget(item); if err != nil { return nil, err }; defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second); defer cancel()
	index, _ := strconv.Atoi(strings.TrimSpace(target.DBName)); client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(target.Host, strconv.Itoa(databasePortByType("redis", target.Port))), Username: target.Username, Password: target.Password, DB: index}); defer client.Close()
	values := map[string]float64{}
	for _, section := range []string{"stats", "clients", "server", "memory"} { info, infoErr := client.Info(ctx, section).Result(); if infoErr != nil { return nil, infoErr }; for _, line := range strings.Split(info, "\n") { pair := strings.SplitN(strings.TrimSpace(line), ":", 2); if len(pair) == 2 { values[pair[0]], _ = strconv.ParseFloat(pair[1], 64) } } }
	hits, misses := values["keyspace_hits"], values["keyspace_misses"]; hitRate := 0.0; if hits+misses > 0 { hitRate = 100 * hits / (hits + misses) }
	return map[string]float64{"counterQueries": values["total_commands_processed"], "connections": values["connected_clients"], "threads": values["blocked_clients"], "counterBytesIn": values["total_net_input_bytes"], "counterBytesOut": values["total_net_output_bytes"], "counterSlowQueries": values["slowlog_len"], "uptime": values["uptime_in_seconds"], "cacheHitRate": hitRate}, nil
}
