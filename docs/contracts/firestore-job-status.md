# Firestore Job Status Contract

## Path

`workspaces/{workspaceId}/jobs/{jobId}`

Firestore stores transient job progress and UI notification state. Postgres/API
remain the source of truth for workspace, document, tree, and job records.

## Schema

The canonical machine-readable schema is
[`../../contracts/firestore/job-status.schema.json`](../../contracts/firestore/job-status.schema.json).

## Document

| Field | Type | Required | Writer | Notes |
|---|---:|---:|---|---|
| `jobId` | string | yes | api/worker | Job ID. |
| `jobType` | string | yes | api/worker | Job type enum string. |
| `documentId` | string | yes | api/worker | Target document ID. |
| `workspaceId` | string | yes | api/worker | Parent workspace ID. |
| `treeId` | string | yes | api/worker | Tree/workspace root ID. |
| `status` | string | yes | api/worker | `queued`, `running`, `succeeded`, or `failed`. |
| `currentStage` | string | yes | worker | Empty when not in a stage. |
| `progress` | number | no | api/worker | 0-100 UI progress. |
| `message` | string | no | api/worker | Human-readable status message. |
| `errorMessage` | string | yes | worker | Empty unless failed. |
| `suggestedWorkspaceName` | string | no | worker | Rename candidate generated from the document brief. |
| `suggestedWorkspaceNameSource` | string | no | worker | Source of the candidate, currently `brief.topic`. |
| `createdAt` | string | no | api | RFC3339 timestamp. |
| `startedAt` | string | no | worker | RFC3339 timestamp. |
| `updatedAt` | string | yes | api/worker | RFC3339 timestamp. |
| `completedAt` | string | no | worker | RFC3339 timestamp. |

## Current Collections

- Single job subscription: `workspaces/{workspaceId}/jobs/{jobId}`
- Recent jobs subscription: `workspaces/{workspaceId}/jobs`, ordered by `updatedAt desc`

## Notes

- Firestore is not the source of truth. The frontend must confirm durable state
  through API calls when it needs workspace/document/tree data.
- Optional fields may be absent on older job documents.
- Security rules are maintained separately in `firestore.rules`; this document
  describes shape, not authorization.
