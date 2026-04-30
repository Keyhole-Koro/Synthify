# Microservices Architecture Design

This document describes the target architecture for migrating the monolithic Synthify backend into a set of independently deployable microservices. Each service owns its data store and communicates via gRPC or Connect-RPC. The migration is planned over three phases spanning Q3–Q4 2025.

## Service Boundaries

1. **Identity Service** — handles authentication, account management, and JWT issuance. Backed by PostgreSQL.
2. **Document Service** — ingests files, tracks processing jobs, and exposes chunked text via internal gRPC. Uses GCS for blob storage.
3. **Worker Service** — LLM orchestration, embedding generation, knowledge tree synthesis. Stateless; reads from shared DB.
4. **Tree Service** — CRUD for tree_items, alias resolution, path queries. Owns the pgvector index.
5. **Notification Service** — SSE/Firestore push for job status updates. No persistent store.

## Data Flow

Upload → Document Service stores blob in GCS, creates document row.
StartProcessing → API triggers Worker via Connect-RPC.
Worker runs LLM pipeline: extract → chunk → embed → synthesize → persist.
Each chunk is embedded with gemini-embedding-2 (768-dim, MRL) and written to document_chunks with a vector column.
Tree Service receives CREATE_ITEM mutations from Worker and updates tree_items.
Notification Service fans out SSE events to subscribed frontend clients.

## Deployment Model

All services are containerised and deployed on Cloud Run. Each service scales independently. Worker is scaled to zero when idle; cold start budget is 8 seconds. The database connection pool is capped at 10 connections per Worker instance to avoid exhausting Cloud SQL limits during burst scaling.

## Observability

All LLM tool invocations are logged to tool_call_logs with input/output JSON and duration_ms.
Job lifecycle transitions are captured in job_mutation_logs with risk_tier tagging.
Distributed tracing via OpenTelemetry is planned for Phase 2.
Key SLOs: p95 job completion < 60s for documents under 50KB; embedding latency < 2s per chunk.

## Open Questions

- Should the Worker be allowed to create items without a pre-approved execution plan for low-risk tier-0 operations?
- What is the retention policy for job_mutation_logs? Current estimate is 90 days.
- How do we handle documents where OCR quality is below 80% confidence — reject at ingest or flag post-chunking?
- Rate limiting strategy for embedding API calls during burst uploads.
- Inter-service authentication: mTLS or internal JWT? Decision needed before Phase 2 begins.
