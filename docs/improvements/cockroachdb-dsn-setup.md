# Database Connection Improvement: CockroachDB & DSN

## Overview
This document describes the transition from local PostgreSQL to **CockroachDB Serverless** and explains the core concepts of **DSN (Data Source Name)** and its interaction with **DNS** for database connectivity.

## Transition to CockroachDB Serverless
To improve scalability and reduce infrastructure management overhead, we have moved the production database to CockroachDB Serverless.

- **Reasoning**: Serverless architecture allows for automatic scaling and cost-efficiency (pay-as-you-go).
- **Compatibility**: While CockroachDB is wire-compatible with PostgreSQL, we updated our schema to use its native `VECTOR` type instead of `pgvector`.

## Understanding DSN (Data Source Name)
The connection to the database is managed via a **DSN**, which is a standardized string containing all necessary connection parameters.

### DSN Format
The DSN is stored in the `DATABASE_URL` environment variable:
`postgresql://<user>:<password>@<host>:<port>/<db>?sslmode=verify-full`

### Components of the DSN
- **User/Password**: Credentials for authentication.
- **Host (Domain)**: The address of the CockroachDB cluster (e.g., `synthify-xxxx.cockroachlabs.cloud`).
- **Port**: Typically `26257` for CockroachDB.
- **SSL Mode**: Required for secure serverless connections (`verify-full`).

## The Role of DNS (Domain Name System)
While the DSN provides the "label" or "address," **DNS** acts as the "phonebook" to resolve the host domain into an IP address.

1. **Resolution**: The application reads the host from the DSN.
2. **Lookup**: The application (via the OS) asks a DNS server for the IP address of the CockroachDB host.
3. **Direct Connection**: Once the IP is resolved, the application establishes a direct network connection to the CockroachDB server.

## Implementation Details
- **Secret Management**: In GCP, the DSN is stored in **Secret Manager** and injected into Cloud Run as the `DATABASE_URL` environment variable.
- **Go Driver**: We use the `pgx` driver via the `database/sql` standard library, which handles DSN parsing and connection pooling automatically.
- **Local Parity**: The `compose.yaml` has been updated to use a local CockroachDB instance to ensure the development environment matches the production behavior.
