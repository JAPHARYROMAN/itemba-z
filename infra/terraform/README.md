# Terraform foundation

This directory intentionally defines the provider-neutral deployment contract before selecting a cloud. Production infrastructure must add an approved provider module for:

- private networking and controlled egress;
- managed multi-zone PostgreSQL with point-in-time recovery;
- managed Redis and S3-compatible object storage;
- container runtime, load balancer, WAF, and certificate management;
- centralized secret management and key rotation;
- immutable backup retention and cross-account recovery;
- OpenTelemetry log, metric, and trace destinations.

Provider selection is gated by the Tanzania data-protection assessment, cross-border transfer approval where applicable, service availability, recovery capability, support, and total operating cost. No production data may be placed in a region before that decision is recorded.
