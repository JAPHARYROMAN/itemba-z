package postgres

import (
	"context"
	"fmt"
)

// PlatformOperationalMetrics returns only aggregate, bounded-cardinality
// platform signals exposed by the audited SECURITY DEFINER function. It never
// returns tenant, company, actor, document or payload data.
func (s *Store) PlatformOperationalMetrics(ctx context.Context) (map[string]map[string]float64, error) {
	rows, err := s.pool.Query(ctx, `SELECT metric,label,value FROM `+s.schema+`.platform_operational_metrics() ORDER BY metric,label`)
	if err != nil {
		return nil, fmt.Errorf("query platform operational metrics: %w", err)
	}
	defer rows.Close()
	result := make(map[string]map[string]float64)
	for rows.Next() {
		var metric, label string
		var value float64
		if err := rows.Scan(&metric, &label, &value); err != nil {
			return nil, fmt.Errorf("scan platform operational metric: %w", err)
		}
		if result[metric] == nil {
			result[metric] = make(map[string]float64)
		}
		result[metric][label] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate platform operational metrics: %w", err)
	}
	return result, nil
}
