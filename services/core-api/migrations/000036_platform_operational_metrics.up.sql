SET search_path TO itembaz, public;

CREATE FUNCTION platform_operational_metrics()
RETURNS TABLE(metric text, label text, value double precision)
LANGUAGE sql
SECURITY DEFINER
STABLE
SET search_path TO itembaz, pg_temp
AS $$
    SELECT 'itemba_outbox_oldest_unpublished_seconds', '',
           COALESCE(EXTRACT(EPOCH FROM (now() - MIN(occurred_at))), 0)::double precision
      FROM outbox_events WHERE processed_at IS NULL
    UNION ALL
    SELECT 'itemba_outbox_pending', '', COUNT(*)::double precision
      FROM outbox_events WHERE processed_at IS NULL
    UNION ALL
    SELECT 'itemba_integration_backlog', capability, COUNT(*)::double precision
      FROM integration_deliveries
     WHERE status IN ('PENDING', 'IN_FLIGHT', 'RETRY_SCHEDULED', 'DEAD_LETTER')
     GROUP BY capability
    UNION ALL
    SELECT 'itemba_integration_failures_total', delivery.capability, COUNT(*)::double precision
      FROM integration_delivery_attempts attempt
      JOIN integration_deliveries delivery
        ON delivery.tenant_id=attempt.tenant_id AND delivery.company_id=attempt.company_id AND delivery.id=attempt.delivery_id
     WHERE attempt.outcome IN ('RETRY_SCHEDULED', 'DEAD_LETTER')
     GROUP BY delivery.capability
    UNION ALL
    SELECT 'itemba_reconciliation_open_critical', '', COUNT(*)::double precision
      FROM mobile_reconciliation_cases reconciliation_case
     WHERE NOT EXISTS (
         SELECT 1 FROM mobile_reconciliation_resolutions resolution
          WHERE resolution.tenant_id=reconciliation_case.tenant_id
            AND resolution.company_id=reconciliation_case.company_id
            AND resolution.branch_id=reconciliation_case.branch_id
            AND resolution.warehouse_id=reconciliation_case.warehouse_id
            AND resolution.case_id=reconciliation_case.id
     );
$$;

REVOKE ALL ON FUNCTION platform_operational_metrics() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform_operational_metrics() TO itembaz_runtime, itembaz_worker_runtime;
