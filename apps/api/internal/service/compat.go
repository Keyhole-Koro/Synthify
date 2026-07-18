// Package service is a deprecated compatibility facade.
// New code must import apps/api/internal/application.
package service

import application "github.com/synthify/backend/apps/api/internal/application"

type WorkerDispatcher = application.WorkerDispatcher
type ObjectMetadataFetcher = application.ObjectMetadataFetcher

type BillingUsecase = application.BillingUsecase
type BillingProvider = application.BillingProvider
type BillingReconciler = application.BillingReconciler
type BillingServiceDeps = application.BillingServiceDeps
type BillingReconciliationDiff = application.BillingReconciliationDiff

var NewBillingService = application.NewBillingService
var ErrBillingUsageRepoRequired = application.ErrBillingUsageRepoRequired

type DocumentUsecase = application.DocumentUsecase
type DocumentService = application.DocumentService
type DocumentServiceDeps = application.DocumentServiceDeps

var NewDocumentService = application.NewDocumentService

type ItemService = application.ItemService
type ItemServiceDeps = application.ItemServiceDeps

var NewItemService = application.NewItemService

type WorkspaceUsecase = application.WorkspaceUsecase
type WorkspaceService = application.WorkspaceService
type WorkspaceServiceDeps = application.WorkspaceServiceDeps

var NewWorkspaceService = application.NewWorkspaceService

type UserUsecase = application.UserUsecase
type UserBillingUsecase = application.UserBillingUsecase
type UserService = application.UserService
type UserServiceDeps = application.UserServiceDeps
type SignInUserResult = application.SignInUserResult

var NewUserService = application.NewUserService

type TreeUsecase = application.TreeUsecase
type TreeService = application.TreeService
type TreeServiceDeps = application.TreeServiceDeps

var NewTreeService = application.NewTreeService

type DevSeedUsecase = application.DevSeedUsecase
type DevSeedService = application.DevSeedService
type DevSeedServiceDeps = application.DevSeedServiceDeps
type DevSeedResult = application.DevSeedResult

var NewDevSeedService = application.NewDevSeedService
