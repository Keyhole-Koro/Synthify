# API Flows and Interactions

This document visualizes the primary interactions between the Frontend, Backend, and external services like Firebase Auth and Stripe.

## 1. Authentication and User Sync

When a user logs in for the first time or returns, the following flow ensures their profile is synchronized.

```mermaid
sequenceDiagram
    participant F as Frontend (React)
    participant A as Firebase Auth
    participant B as Backend (UserService)
    participant DB as BigQuery

    F->>A: Login (Google OAuth)
    A-->>F: ID Token (JWT)
    F->>B: SyncUser (Header: Authorization: Bearer <Token>)
    B->>B: Verify JWT
    B->>DB: Check/Create User Record
    DB-->>B: User Data
    B-->>F: SyncUserResponse (User info, is_new_user)
```

## 2. Workspace Management & Billing (Stripe)

Upgrading a workspace to the 'Pro' plan involves a redirection to Stripe.

```mermaid
sequenceDiagram
    participant F as Frontend
    participant B as Backend (BillingService)
    participant S as Stripe
    participant DB as BigQuery (workspaces)

    F->>B: CreateCheckoutSession(workspace_id)
    B->>S: Create Session API
    S-->>B: Checkout URL
    B-->>F: Returns URL
    F->>S: User completes payment on Stripe
    S-->>B: Webhook (checkout.session.completed)
    B->>B: Verify Signature
    B->>DB: Update workspace plan to 'pro'
```

## 3. Interactive Graph Exploration

The core value of the system is the interactive traversal of the knowledge graph via the paper-in-paper UI.

```mermaid
sequenceDiagram
    participant F as Frontend (React + paper-in-paper)
    participant G as Backend (GraphService)
    participant N as Backend (NodeService)
    participant BQ as BigQuery
    participant SG as Spanner Graph

    F->>G: GetGraph(workspace_id, document_id)
    G->>BQ: Query document graph
    BQ-->>G: document nodes
    G-->>F: GetGraphResponse (nodes)
    Note over F: Build PaperMap from node metadata and summary_html links

    Note over F, SG: User clicks data-paper-id link inside a Paper's iframe content
    Note over F: paper-in-paper fires OPEN_NODE → expand target Paper inline

    F->>G: FindPaths(source_node_id, target_node_id)
    G->>SG: Path query
    SG-->>G: GraphPath + PathEvidenceRef
    G-->>F: FindPathsResponse (node_ids sequence)
    Note over F: Translate path node_ids → sequential OPEN_NODE operations

    F->>N: GetGraphEntityDetail(target_ref)
    N->>BQ: Load source chunks / representative nodes
    BQ-->>N: GraphEntityDetail evidence
    N-->>F: GetGraphEntityDetailResponse
    Note over F: Display in metadata panel (right slide-in)
```

## 4. Monitoring and Metrics Families

`/dev/stats` is organized by metrics family so the UI, RPCs, and stored aggregates use the same vocabulary.

```mermaid
flowchart LR
    A[/dev/stats/] --> B[PipelineMetrics]
    A --> C[ExtractionMetrics]
    A --> D[EvaluationMetrics]
    A --> E[ErrorMetrics]
    A --> F[NormalizationMetrics]
    A --> G[AliasMetrics]

    B --> B1[GetPipelineStats]
    C --> C1[GetExtractionStats]
    D --> D1[GetEvaluationTrend]
    E --> E1[ListFailedDocuments]
    F --> F1[ListNormalizationTools / GetNormalizationToolRun]
    G --> G1[GetAliasStats]
```
