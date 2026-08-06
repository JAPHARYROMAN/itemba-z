package telemetry

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var latencyBuckets = [...]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30}
var operationalLabelPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

type metricKey struct {
	method      string
	route       string
	statusClass string
}

type latencyHistogram struct {
	count   uint64
	sum     float64
	buckets [len(latencyBuckets)]uint64
}

// Registry is an in-process, bounded-cardinality Prometheus registry for the
// core HTTP service. Labels never contain tenant, actor, document or raw path.
type Registry struct {
	mu       sync.RWMutex
	requests map[metricKey]uint64
	latency  map[metricKey]latencyHistogram
	database struct {
		acquired int32
		idle     int32
		total    int32
		maximum  int32
		set      bool
	}
	operational map[string]map[string]float64
}

func (registry *Registry) UpdateDatabasePool(acquired, idle, total, maximum int32) {
	if registry == nil || acquired < 0 || idle < 0 || total < 0 || maximum < 1 {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.database.acquired, registry.database.idle = acquired, idle
	registry.database.total, registry.database.maximum, registry.database.set = total, maximum, true
}

func NewRegistry() *Registry {
	return &Registry{requests: make(map[metricKey]uint64), latency: make(map[metricKey]latencyHistogram), operational: make(map[string]map[string]float64)}
}

func (registry *Registry) ReplaceOperationalMetrics(metrics map[string]map[string]float64) {
	if registry == nil {
		return
	}
	allowed := map[string]bool{
		"itemba_outbox_oldest_unpublished_seconds": true,
		"itemba_outbox_pending":                    true,
		"itemba_integration_backlog":               true,
		"itemba_integration_failures_total":        true,
		"itemba_reconciliation_open_critical":      true,
	}
	clean := make(map[string]map[string]float64)
	for name, values := range metrics {
		if !allowed[name] {
			continue
		}
		clean[name] = make(map[string]float64)
		for label, value := range values {
			if (label == "" || operationalLabelPattern.MatchString(label)) && value >= 0 {
				clean[name][label] = value
			}
		}
	}
	registry.mu.Lock()
	registry.operational = clean
	registry.mu.Unlock()
}

func (registry *Registry) ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	if registry == nil {
		return
	}
	if route == "" {
		route = "unmatched"
	}
	key := metricKey{method: method, route: route, statusClass: strconv.Itoa(status/100) + "xx"}
	seconds := duration.Seconds()
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.requests[key]++
	histogram := registry.latency[key]
	histogram.count++
	histogram.sum += seconds
	for index, boundary := range latencyBuckets {
		if seconds <= boundary {
			histogram.buckets[index]++
		}
	}
	registry.latency[key] = histogram
}

func (registry *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/metrics" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-store")
		_, _ = writer.Write([]byte(registry.render()))
	})
}

func (registry *Registry) render() string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	keys := make([]metricKey, 0, len(registry.requests))
	for key := range registry.requests {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].method+keys[i].route+keys[i].statusClass < keys[j].method+keys[j].route+keys[j].statusClass
	})
	var result strings.Builder
	result.WriteString("# HELP itemba_http_requests_total Completed HTTP requests.\n# TYPE itemba_http_requests_total counter\n")
	for _, key := range keys {
		labels := metricLabels(key)
		fmt.Fprintf(&result, "itemba_http_requests_total{%s} %d\n", labels, registry.requests[key])
	}
	result.WriteString("# HELP itemba_http_request_duration_seconds HTTP request latency.\n# TYPE itemba_http_request_duration_seconds histogram\n")
	for _, key := range keys {
		histogram := registry.latency[key]
		labels := metricLabels(key)
		for index, boundary := range latencyBuckets {
			fmt.Fprintf(&result, "itemba_http_request_duration_seconds_bucket{%s,le=%q} %d\n", labels, strconv.FormatFloat(boundary, 'f', -1, 64), histogram.buckets[index])
		}
		fmt.Fprintf(&result, "itemba_http_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", labels, histogram.count)
		fmt.Fprintf(&result, "itemba_http_request_duration_seconds_sum{%s} %g\n", labels, histogram.sum)
		fmt.Fprintf(&result, "itemba_http_request_duration_seconds_count{%s} %d\n", labels, histogram.count)
	}
	result.WriteString("# HELP itemba_build_info Static runtime identity.\n# TYPE itemba_build_info gauge\nitemba_build_info{service=\"core-api\"} 1\n")
	if registry.database.set {
		result.WriteString("# HELP itemba_database_pool_connections Current database pool connections.\n# TYPE itemba_database_pool_connections gauge\n")
		fmt.Fprintf(&result, "itemba_database_pool_connections{state=\"acquired\"} %d\nitemba_database_pool_connections{state=\"idle\"} %d\nitemba_database_pool_connections{state=\"total\"} %d\n", registry.database.acquired, registry.database.idle, registry.database.total)
		result.WriteString("# HELP itemba_database_pool_max_connections Configured maximum database pool connections.\n# TYPE itemba_database_pool_max_connections gauge\n")
		fmt.Fprintf(&result, "itemba_database_pool_max_connections %d\n", registry.database.maximum)
	}
	for _, name := range []string{"itemba_outbox_oldest_unpublished_seconds", "itemba_outbox_pending", "itemba_integration_backlog", "itemba_integration_failures_total", "itemba_reconciliation_open_critical"} {
		values := registry.operational[name]
		if len(values) == 0 {
			continue
		}
		metricType := "gauge"
		if name == "itemba_integration_failures_total" {
			metricType = "counter"
		}
		fmt.Fprintf(&result, "# TYPE %s %s\n", name, metricType)
		labels := make([]string, 0, len(values))
		for label := range values {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		for _, label := range labels {
			if label == "" {
				fmt.Fprintf(&result, "%s %g\n", name, values[label])
			} else {
				fmt.Fprintf(&result, "%s{capability=%q} %g\n", name, escapeLabel(label), values[label])
			}
		}
	}
	return result.String()
}

func metricLabels(key metricKey) string {
	return fmt.Sprintf("method=%q,route=%q,status_class=%q", escapeLabel(key.method), escapeLabel(key.route), escapeLabel(key.statusClass))
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
