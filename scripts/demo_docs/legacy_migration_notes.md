# Legacy System Migration — Internal Notes

## Background

The legacy monolith (codename: KRONOS) has been running since 2017 on a bare-metal PostgreSQL 9.6 instance. The codebase is approximately 180k lines of PHP 5.6 with no test coverage. Migration to the new Go stack is blocked by three issues: undocumented stored procedures that implement business logic, binary blobs stored directly in the DB as bytea columns, and a custom session management scheme that predates JWT.

## Risk Register

HIGH: Data loss during bytea to GCS migration. Mitigation: run dual-write for 30 days, validate checksums.
HIGH: Stored procedure reverse-engineering may miss edge cases. Mitigation: shadow mode — run old and new code in parallel, diff outputs.
MEDIUM: PHP session tokens still active in production. Plan: force re-authentication window of 72h during cutover weekend.
LOW: Reporting queries rely on PostgreSQL 9.6-specific syntax. Mitigation: regression test suite against PG16 before cutover.

## Cutover Plan

Week 1: Enable dual-write. All writes go to both KRONOS DB and new service DB.
Week 2-4: Backfill historical data. Estimated 40 million rows across 12 tables.
Week 5: Shadow mode for reads. Compare response payloads between old and new APIs.
Week 6: Cutover weekend. DNS flip, KRONOS set to read-only.
Week 7: Monitor error rates. Rollback trigger: greater than 0.5% 5xx rate on any endpoint for 5 or more minutes.
Week 8: Decommission KRONOS. Archive DB snapshot to cold storage.

## Known Technical Debt

The KRONOS session table has 220 million rows and no TTL mechanism. A one-time purge job must be written and tested before the backfill begins. Estimated purge time at 100k rows/s is approximately 37 minutes; this needs to run during a low-traffic window.

Three stored procedures (sp_calculate_billing, sp_generate_invoice, sp_apply_discount) have no corresponding documentation. They must be reverse-engineered from query plans and production logs before the migration team can reimplement them in Go.
