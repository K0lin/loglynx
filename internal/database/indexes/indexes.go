package indexes

import (
	"strings"

	"github.com/pterm/pterm"
	"gorm.io/gorm"
)

// Definition represents an index name and its creation SQL.
type Definition struct {
	Name string
	SQL  string
}

// expectedDefinitions is the single source of truth for performance indexes.
var expectedDefinitions = []Definition{
	// ===== UNIQUE CONSTRAINT FOR DEDUPLICATION =====
	{Name: "idx_request_hash", SQL: `CREATE UNIQUE INDEX IF NOT EXISTS idx_request_hash ON http_requests(request_hash)`},

	// ===== PRIMARY COMPOSITE INDEXES (for time-range queries) =====
	{Name: "idx_timestamp_status", SQL: `CREATE INDEX IF NOT EXISTS idx_timestamp_status ON http_requests(timestamp DESC, status_code)`},
	{Name: "idx_time_host", SQL: `CREATE INDEX IF NOT EXISTS idx_time_host ON http_requests(timestamp DESC, host)`},
	{Name: "idx_time_backend", SQL: `CREATE INDEX IF NOT EXISTS idx_time_backend ON http_requests(timestamp DESC, backend_name, status_code)`},
	{Name: "idx_time_backend_url", SQL: `CREATE INDEX IF NOT EXISTS idx_time_backend_url ON http_requests(timestamp DESC, backend_url)`},
	{Name: "idx_time_client_ip", SQL: `CREATE INDEX IF NOT EXISTS idx_time_client_ip ON http_requests(timestamp DESC, client_ip)`},
	{Name: "idx_summary_cover", SQL: `CREATE INDEX IF NOT EXISTS idx_summary_cover ON http_requests(status_code, response_size, response_time_ms, client_ip, path)`},

	// ===== AGGREGATION INDEXES (for GROUP BY queries) =====
	{Name: "idx_path_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_path_agg ON http_requests(path, timestamp, client_ip, response_time_ms, response_size)`},
	{Name: "idx_top_paths_cover", SQL: `CREATE INDEX IF NOT EXISTS idx_top_paths_cover ON http_requests(timestamp DESC, path, client_ip, response_size, response_time_ms)`},
	{Name: "idx_geo_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_geo_agg ON http_requests(geo_country, timestamp, client_ip, response_size) WHERE geo_country != ''`},
	{Name: "idx_referer_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_referer_agg ON http_requests(referer, timestamp, client_ip) WHERE referer != ''`},
	{Name: "idx_service_id", SQL: `CREATE INDEX IF NOT EXISTS idx_service_id ON http_requests(backend_name, backend_url, host)`},
	{Name: "idx_backend_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_backend_agg ON http_requests(backend_name, timestamp, backend_url, host, response_size, status_code) WHERE backend_name != ''`},
	{Name: "idx_backend_url_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_backend_url_agg ON http_requests(backend_url, timestamp, host, response_size, status_code) WHERE backend_name = '' AND backend_url != ''`},
	{Name: "idx_host_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_host_agg ON http_requests(host, timestamp, response_size, status_code) WHERE backend_name = '' AND backend_url = '' AND host != ''`},

	// ===== LOOKUP INDEXES (for filtering and detail queries) =====
	{Name: "idx_ip_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_ip_agg ON http_requests(client_ip, timestamp, geo_country, geo_city, geo_lat, geo_lon, response_size, asn, asn_org, status_code)`},
	{Name: "idx_top_ips_cover", SQL: `CREATE INDEX IF NOT EXISTS idx_top_ips_cover ON http_requests(client_ip, timestamp DESC, geo_country, geo_city, geo_lat, geo_lon, response_size, status_code)`},
	{Name: "idx_ip_browser_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_ip_browser_agg ON http_requests(client_ip, timestamp, browser) WHERE browser != ''`},
	{Name: "idx_ip_backend_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_ip_backend_agg ON http_requests(client_ip, timestamp, backend_name, backend_url, response_size, response_time_ms, status_code) WHERE backend_name != ''`},
	{Name: "idx_ip_device_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_ip_device_agg ON http_requests(client_ip, timestamp, device_type) WHERE device_type != ''`},
	{Name: "idx_ip_os_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_ip_os_agg ON http_requests(client_ip, timestamp, os) WHERE os != ''`},
	{Name: "idx_ip_status_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_ip_status_agg ON http_requests(client_ip, timestamp, status_code)`},
	{Name: "idx_ip_path_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_ip_path_agg ON http_requests(client_ip, timestamp, path, response_time_ms, response_size, backend_name, host, backend_url)`},
	{Name: "idx_ip_heatmap_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_ip_heatmap_agg ON http_requests(client_ip, timestamp, response_time_ms, response_size, backend_name)`},
	{Name: "idx_status_code", SQL: `CREATE INDEX IF NOT EXISTS idx_status_code ON http_requests(status_code, timestamp)`},
	{Name: "idx_method", SQL: `CREATE INDEX IF NOT EXISTS idx_method ON http_requests(method, timestamp)`},
	{Name: "idx_asn_agg", SQL: `CREATE INDEX IF NOT EXISTS idx_asn_agg ON http_requests(asn, timestamp, asn_org, geo_country, response_size) WHERE asn > 0`},
	{Name: "idx_device_type", SQL: `CREATE INDEX IF NOT EXISTS idx_device_type ON http_requests(device_type, timestamp) WHERE device_type != ''`},
	{Name: "idx_protocol", SQL: `CREATE INDEX IF NOT EXISTS idx_protocol ON http_requests(protocol, timestamp) WHERE protocol != ''`},
	{Name: "idx_tls_version", SQL: `CREATE INDEX IF NOT EXISTS idx_tls_version ON http_requests(tls_version, timestamp) WHERE tls_version != ''`},

	// ===== PARTIAL INDEXES (for specific filtered queries) =====
	{Name: "idx_errors", SQL: `CREATE INDEX IF NOT EXISTS idx_errors ON http_requests(timestamp DESC, status_code, path, client_ip) WHERE status_code >= 400`},
	{Name: "idx_slow", SQL: `CREATE INDEX IF NOT EXISTS idx_slow ON http_requests(timestamp DESC, response_time_ms, path, host) WHERE response_time_ms > 1000`},
	{Name: "idx_response_time", SQL: `CREATE INDEX IF NOT EXISTS idx_response_time ON http_requests(timestamp DESC, response_time_ms) WHERE response_time_ms > 0`},

	// ===== MAINTENANCE INDEX =====
	{Name: "idx_cleanup", SQL: `CREATE INDEX IF NOT EXISTS idx_cleanup ON http_requests(timestamp)`},
}

// legacyIndexes represent deprecated/older index names that should be dropped when reconciling.
var legacyIndexes = []string{
	"idx_source_name",
	"idx_timestamp",
	"idx_partition_key",
	"idx_client_ip",
	"idx_host",
	"idx_status",
	"idx_response_time",
	"idx_retry_attempts",
	"idx_browser",
	"idx_os",
	"idx_device_type",
	"idx_router_name",
	"idx_request_id",
	"idx_trace_id",
	"idx_geo_country",
	"idx_created_at",
	"idx_time_status",
	"idx_ip_time",
	"idx_status_host",
	"idx_timestamp_response_time",
	"idx_summary_query",
	"idx_errors_only",
	"idx_slow_requests",
	"idx_server_errors",
	"idx_retried_requests",
	"idx_dashboard_covering",
	"idx_error_analysis",
	"idx_timestamp_cleanup",
}

// Ensure reconciles expected indexes against SQLite, dropping obsolete ones and creating missing ones.
func Ensure(db *gorm.DB, logger *pterm.Logger) (created int, dropped int, err error) {
	existingIndexes, err := fetchExistingIndexes(db)
	if err != nil {
		return 0, 0, err
	}

	existingSet := make(map[string]struct{}, len(existingIndexes))
	for _, name := range existingIndexes {
		existingSet[name] = struct{}{}
	}

	expectedSet := make(map[string]Definition, len(expectedDefinitions))
	for _, def := range expectedDefinitions {
		expectedSet[def.Name] = def
	}

	var unexpected []string
	for name := range existingSet {
		if _, ok := expectedSet[name]; !ok {
			unexpected = append(unexpected, name)
		}
	}

	var missing []Definition
	for _, def := range expectedDefinitions {
		if _, ok := existingSet[def.Name]; !ok {
			missing = append(missing, def)
		}
	}

	hasMismatch := len(unexpected) > 0 || len(missing) > 0
	if hasMismatch {
		namesToDrop := uniqueNames(append(legacyIndexes, unexpected...))
		for _, name := range namesToDrop {
			if err := db.Exec("DROP INDEX IF EXISTS " + name).Error; err != nil {
				logger.Warn("Failed to drop index", logger.Args("index", name, "error", err))
				continue
			}
			dropped++
		}
	}

	for _, def := range expectedDefinitions {
		if err := db.Exec(def.SQL).Error; err != nil {
			logger.Warn("Failed to create index", logger.Args("index", def.Name, "error", err))
			return created, dropped, err
		}
		if _, ok := existingSet[def.Name]; !ok {
			created++
		}
	}

	return created, dropped, nil
}

func fetchExistingIndexes(db *gorm.DB) ([]string, error) {
	var names []string
	rows, err := db.Raw(`SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='http_requests' AND name NOT LIKE 'sqlite_%'`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func uniqueNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}
