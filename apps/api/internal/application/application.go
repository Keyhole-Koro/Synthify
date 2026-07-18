// Package application exposes the API application's use-case layer.
package application

import service "github.com/synthify/backend/apps/api/internal/application/impl"

type WorkerDispatcher = service.WorkerDispatcher
type ObjectMetadataFetcher = service.ObjectMetadataFetcher

type BillingUsecase = service.BillingUsecase
type BillingProvider = service.BillingProvider
type BillingReconciler = service.BillingReconciler
type BillingServiceDeps = service.BillingServiceDeps
type BillingReconciliationDiff = service.BillingReconciliationDiff

var NewBillingService = service.NewBillingService
var ErrBillingUsageRepoRequired = service.ErrBillingUsageRepoRequired

type DocumentUsecase = service.DocumentUsecase
type DocumentService = service.DocumentService
type DocumentServiceDeps = service.DocumentServiceDeps

var NewDocumentService = service.NewDocumentService

type ItemService = service.ItemService
type ItemServiceDeps = service.ItemServiceDeps

var NewItemService = service.NewItemService

type WorkspaceUsecase = service.WorkspaceUsecase
type WorkspaceService = service.WorkspaceService
type WorkspaceServiceDeps = service.WorkspaceServiceDeps

var NewWorkspaceService = service.NewWorkspaceService

type UserUsecase = service.UserUsecase
type UserBillingUsecase = service.UserBillingUsecase
type UserService = service.UserService
type UserServiceDeps = service.UserServiceDeps
type SignInUserResult = service.SignInUserResult

var NewUserService = service.NewUserService

type TreeUsecase = service.TreeUsecase
type TreeService = service.TreeService
type TreeServiceDeps = service.TreeServiceDeps

var NewTreeService = service.NewTreeService

type DevSeedUsecase = service.DevSeedUsecase
type DevSeedService = service.DevSeedService
type DevSeedServiceDeps = service.DevSeedServiceDeps
type DevSeedResult = service.DevSeedResult

var NewDevSeedService = service.NewDevSeedService
