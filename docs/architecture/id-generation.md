# ID generation in production

## Scope

This document describes IDs used by the production code paths: PostgreSQL repositories, worker execution, and persisted worker outputs.

The mock repository intentionally uses simple readable IDs for tests and local fakes. Mock IDs are out of scope here and must not be used as evidence for production uniqueness.

## Primary rule

Production persisted entity IDs should be globally unique unless this document explicitly marks them as derived IDs or local-scope IDs.

PostgreSQL repository code uses `newID()` as the default persisted ID generator:

```go
func newID() string {
    return ulid.Make().String()
}
```

`newID()` returns a ULID string. These IDs are suitable for primary keys such as workspaces, documents, jobs, items, and mutation logs.

## ULID-backed persisted IDs

The following production IDs are generated with `newID()` and are intended to be globally unique.

| Entity | DB column | Generation site | Notes |
|---|---|---|---|
| Account | `accounts.account_id` | `GetOrCreateAccount` | New account ID is ULID. Existing Firebase user is mapped through `account_users.user_id`. |
| Workspace | `workspaces.workspace_id` | `CreateWorkspace` | ULID. |
| Workspace root item | `tree_items.id` | `CreateWorkspace` | ULID created alongside workspace. |
| Document | `documents.document_id` | `CreateDocument` | ULID. |
| Processing job | `document_processing_jobs.job_id` | `CreateProcessingJob` | ULID. Multiple jobs can exist for the same document. |
| Tree item | `tree_items.id` | `CreateItem`, `CreateStructuredItemWithCapability` | ULID. |
| Job mutation log | `job_mutation_logs.mutation_id` | item mutations and tool-call logging | ULID. |
| Approval request | `job_approval_requests.approval_id` | `RequestJobApproval` | `apr_` prefix plus ULID. |

## Derived persisted IDs

Some persisted IDs are deterministic derivatives of a ULID-backed parent.

### Capability ID

`JobCapability.CapabilityID` is:

```text
cap_{job_id}
```

Because `job_id` is ULID-backed and one capability is created per job, this is unique under the current model.

If the system later supports multiple capabilities per job, this must change to a separate ULID-backed ID or include a capability version/role suffix.

### Execution plan ID

The default execution plan ID is:

```text
plan_{job_id}
```

`CreateProcessingJob` creates the initial plan with this ID. `UpsertJobExecutionPlan` also fills an empty `PlanID` as `plan_{job_id}`.

This is unique under the current model of one primary execution plan per job. If the system later supports multiple candidate plans, plan revisions, or router-generated subplans for the same job, plan IDs must become ULID-backed or include a revision identifier.

## Document chunk IDs

`document_chunks.chunk_id` is currently the primary key, not `(document_id, chunk_id)`.

Chunk IDs are generated in two forms:

```text
{document_id}_chunk_{index}
chk_{document_id}_{left_padded_index}
```

Because production `document_id` is ULID-backed, these are globally unique as long as every chunk ID includes the document ID.

Important constraints:

- Do not create production chunk IDs that are only `chunk_{index}`.
- Do not change `document_chunks.chunk_id` generation without considering that the DB primary key is `chunk_id` alone.
- Re-chunking the same document intentionally reuses chunk IDs by index after deleting existing chunks. This makes chunk IDs stable for a given document/index pair but not stable across text changes.

If the system needs multiple chunking versions for the same document, use a chunking version in the ID or change the schema to include a versioned composite key.

## Item source keys

`item_sources` does not have a standalone generated ID. Its identity is the composite primary key:

```text
(item_id, document_id, chunk_id)
```

This is intentionally an upsert key. Re-saving the same item/document/chunk source updates the row rather than creating a new source row.

## Worker local IDs

`domain.SynthesizedItem.LocalID` is not a persisted DB ID.

It is a temporary ID used inside one synthesis result so `parent_local_id` can refer to another generated item before database IDs exist.

Examples:

```text
item_1
item_2
chunk_0
chunk_1
```

Production persistence maps local IDs to real `tree_items.id` values after each item is created:

```text
local_id -> tree_items.id
```

Required behavior:

- `local_id` only needs to be unique within a single synthesis payload.
- Before persistence, duplicate `local_id` values should be rejected or normalized, because duplicate local IDs can break `parent_local_id` resolution.
- `local_id` must not be stored as `tree_items.id`.

## HTML content references

Generated summaries may include links such as:

```html
<a data-paper-id="{local_id}">...</a>
```

During synthesis, `{local_id}` refers to temporary local IDs. For persisted content, these references should be rewritten to real `tree_items.id` values once the local-to-persisted ID mapping is known.

If this rewrite is missing, links may point to non-persisted local IDs such as `item_1` or `chunk_0`.

## Known watch points

- `plan_{job_id}` and `cap_{job_id}` assume one plan and one capability per job.
- `document_chunks.chunk_id` assumes chunk IDs include the globally unique document ID.
- `SynthesizedItem.LocalID` is local-scope only and needs uniqueness validation before persistence.
- HTML `data-paper-id` values need a local-to-persisted ID rewrite if they are generated with local IDs.
