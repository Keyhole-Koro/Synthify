// Package application exposes the API application's use-case layer.
//
// The implementation is currently migrated from the legacy service package in
// small, behavior-preserving steps. New handlers and bootstrap wiring should
// depend on this package rather than importing service directly.
package application

import "github.com/synthify/backend/apps/api/internal/service"

type WorkerDispatcher = service.WorkerDispatcher

type BillingService = service.BillingService
type BillingServiceDeps = service.BillingServiceDeps

var NewBillingService = service.NewBillingService

type DocumentService = service.DocumentService
type DocumentServiceDeps = service.DocumentServiceDeps

var NewDocumentService = service.NewDocumentService

type ItemService = service.ItemService
type ItemServiceDeps = service.ItemServiceDeps

var NewItemService = service.NewItemService

type WorkspaceService = service.WorkspaceService
type WorkspaceServiceDeps = service.WorkspaceServiceDeps

var NewWorkspaceService = service.NewWorkspaceService

type UserService = service.UserService
type UserServiceDeps = service.UserServiceDeps

var NewUserService = service.NewUserService

type TreeService = service.TreeService
type TreeServiceDeps = service.TreeServiceDeps

var NewTreeService = service.NewTreeService

type DevSeedService = service.DevSeedService
type DevSeedServiceDeps = service.DevSeedServiceDeps

var NewDevSeedService = service.NewDevSeedService
