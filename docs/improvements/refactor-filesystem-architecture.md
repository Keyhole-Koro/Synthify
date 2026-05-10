# Refactor: FileSystem Architecture

## Overview
Replace the redundant and implementation-specific `sourcefiles.FUSE` (and `FUSEHandler`) with a more intuitive and architecturally sound `FileSystem` structure integrated into the Worker's common context.

## Goals
- Rename `FUSEHandler` to `FileSystem` to reflect its role as a generic storage abstraction.
- Remove the global `sourcefiles.FUSE` variable.
- Inject `FileSystem` into `base.Context` to allow tools to access it via `b.FS`.
- Clean up method names (e.g., `ResolvePath` -> `DocPath`).
- Ensure no backward compatibility is maintained (clean break).

## Tasks

### 1. Shared Storage Refactor
- [ ] Rename `packages/shared/storage/fuse.go` to `packages/shared/storage/filesystem.go`.
- [ ] Rename `FUSEHandler` struct to `FileSystem`.
- [ ] Update methods:
    - `NewFUSEHandler` -> `NewFileSystem`
    - `ResolvePath` -> `DocPath`
    - `ResolveCachePath` -> `CachePath`
    - `ResolveCheckpointPath` -> `CheckpointPath`

### 2. Base Context Update
- [ ] Add `FS *storage.FileSystem` to `base.Context` in `apps/worker/pkg/worker/tools/base/context.go`.

### 3. Worker Initialization
- [ ] Update `apps/worker/cmd/server/main.go` to initialize `FileSystem` and pass it to the orchestrator/context.
- [ ] Update `Orchestrator` in `apps/worker/pkg/worker/agents/orchestrator.go` to use `FileSystem`.

### 4. Tool Refactoring
- [ ] **Extraction Tool**: Update `apps/worker/pkg/worker/tools/io/extraction.go` to use `b.FS`.
- [ ] **Grep Tool**: Update `apps/worker/pkg/worker/tools/io/grep.go` to use `b.FS`.
- [ ] **Persistence Tool**: Update `apps/worker/pkg/worker/tools/io/persistence.go` to use `b.FS`.
- [ ] Update related tests.

### 5. Cleanup
- [ ] Remove `FUSE` variable from `apps/worker/pkg/worker/sourcefiles/fetch.go`.
- [ ] Update `Fetch` to use `FileSystem` if passed or from a local reference.
