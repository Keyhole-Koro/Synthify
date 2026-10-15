package bootstrap

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	gcs "cloud.google.com/go/storage"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/api/internal/config"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	"github.com/synthify/backend/apps/api/internal/repository/postgres"
	"github.com/synthify/backend/internal/platform/database"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
	"github.com/synthify/backend/internal/platform/storage"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
)

// AppContext aggregates the core storage engine and job status notifier initialized during bootstrap.
type AppContext struct {
	Store    Store
	Notifier jobstatus.Notifier
}

// Bootstrap initializes the persistent data store and job status notification mechanisms.
func Bootstrap(ctx context.Context, gcsBucket, gcsURLBase, firebaseProjectID string, logger *slog.Logger, nrApp *newrelic.Application) *AppContext {
	store := InitStore(ctx, NewDocumentUploadURLIssuer(gcsBucket, gcsURLBase), logger, nrApp)
	notifier := jobstatus.NewNotifier(ctx, firebaseProjectID, logger)
	return &AppContext{
		Store:    store,
		Notifier: notifier,
	}
}

// Store composite interface gathers all domain repository and transaction interfaces required by the API server.
type Store interface {
	repository.AccountRepository
	repository.WorkspaceRepository
	repository.DocumentRepository
	repository.DocumentChunkRepository
	repository.JobRepository
	repository.JobApprovalRepository
	repository.JobLogRepository
	repository.TreeRepository
	repository.ItemRepository
	repository.UsageRepository
	repository.UserRepository
	repository.CheckpointRepository
	repository.JobLogWriter
	repository.DynamicToolRepository
	repository.WorkspaceChatRepository
	repository.Transactor
}

// InitStore configures and connects to the PostgreSQL store or in-memory mock store depending on configuration.
func InitStore(ctx context.Context, uploadURLIssuer repository.DocumentUploadURLIssuer, logger *slog.Logger, nrApp *newrelic.Application) Store {
	storeCfg := config.LoadStore()
	if dsn := storeCfg.DatabaseDSN; dsn != "" {
		pool := database.PoolConfig{
			MaxOpenConns: storeCfg.DBMaxOpenConns,
			MaxIdleConns: storeCfg.DBMaxIdleConns,
		}
		var lastErr error
		for attempt := 1; attempt <= 10; attempt++ {
			store, err := postgres.NewStore(ctx, dsn, pool, uploadURLIssuer, logger, nrApp)
			if err == nil {
				logger.Info("api.store_initialized", "type", "postgres")
				return store
			}
			lastErr = err
			logger.Warn("api.store_init_retry", "error", err.Error(), "attempt", attempt)
			time.Sleep(2 * time.Second)
		}
		// The database is unreachable after all retries (e.g. it is down or a
		// serverless cluster is disabled). Do NOT panic: a crash loop takes the
		// whole API down — including the liveness probe — so Cloud Run never
		// serves and every request becomes a 503 / browser NetworkError. Instead
		// start with a lazily-connecting store: the process stays up, /health
		// passes, and DB-backed RPCs recover automatically once the database is
		// reachable again. DB-independent endpoints keep working throughout.
		logger.Error("api.store_init_degraded", "error", lastErr.Error())
		store, err := postgres.NewStoreWithoutPing(dsn, pool, uploadURLIssuer, logger, nrApp)
		if err != nil {
			// Only a malformed DSN reaches here (connection is lazy); that is a
			// genuine config error with no recovery, so fail fast.
			logger.Error("api.store_init_failed", "error", err.Error())
			panic(err)
		}
		return store
	}
	logger.Info("api.store_initialized", "type", "mock")
	return mock.NewStore()
}

// NewDocumentUploadURLIssuer constructs a DocumentUploadURLIssuer using signed GCS URLs or local emulator endpoints.
func NewDocumentUploadURLIssuer(bucket, base string) repository.DocumentUploadURLIssuer {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GCS_UPLOAD_ISSUER")))
	if mode == "signed" {
		return NewGCSSignedDocumentUploadURLIssuer(
			bucket,
			os.Getenv("GCS_SIGNING_SERVICE_ACCOUNT_EMAIL"),
			os.Getenv("GCS_SIGNING_PRIVATE_KEY"),
			uploadSignedURLTTL(),
		)
	}
	return NewFakeGCSDocumentUploadURLIssuer(base, bucket)
}

// NewDocumentSourceURLBuilder returns a DocumentSourceURLBuilder constructing access URLs for document sources.
func NewDocumentSourceURLBuilder(bucket, base string) repository.DocumentSourceURLBuilder {
	return func(workspaceID, documentID string) string {
		return storage.BuildDocumentSourceURL(base, bucket, workspaceID, documentID)
	}
}

// NewDocumentImageURLIssuer mirrors NewDocumentUploadURLIssuer: a signed GCS GET
// URL in production, a direct fake-gcs media URL locally. The signed variant is
// selected by the same GCS_UPLOAD_ISSUER=signed switch so prod and dev stay in
// lockstep with the upload path.
func NewDocumentImageURLIssuer(bucket, base string) repository.DocumentImageURLIssuer {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GCS_UPLOAD_ISSUER")))
	if mode == "signed" {
		return newGCSSignedDocumentImageURLIssuer(
			bucket,
			os.Getenv("GCS_SIGNING_SERVICE_ACCOUNT_EMAIL"),
			os.Getenv("GCS_SIGNING_PRIVATE_KEY"),
			uploadSignedURLTTL(),
		)
	}
	return fakeGCSDocumentImageURLIssuer{base: base, bucket: bucket}
}

type fakeGCSDocumentImageURLIssuer struct {
	base   string
	bucket string
}

// IssueDocumentImageURL generates a direct HTTP media URL for local fake-GCS storage.
func (i fakeGCSDocumentImageURLIssuer) IssueDocumentImageURL(ctx context.Context, workspaceID, documentID, filename string) (repository.SignedURL, error) {
	// fake-gcs needs no signature; serve the object directly via its media URL.
	objectName := storage.SourceObjectName(documentID, filename)
	root := strings.TrimRight(i.base, "/")
	url := fmt.Sprintf("%s/storage/v1/b/%s/o/%s?alt=media",
		root, i.bucket, neturl.PathEscape(workspaceID+"/"+objectName))
	return repository.SignedURL{URL: url}, nil
}

type gcsSignedDocumentImageURLIssuer struct {
	bucket         string
	googleAccessID string
	privateKey     []byte
	ttl            time.Duration
}

// newGCSSignedDocumentImageURLIssuer initializes a signed URL issuer for document images in Cloud Storage.
func newGCSSignedDocumentImageURLIssuer(bucket, googleAccessID, privateKey string, ttl time.Duration) gcsSignedDocumentImageURLIssuer {
	privateKey = strings.ReplaceAll(privateKey, `\n`, "\n")
	var key []byte
	if privateKey != "" {
		key = []byte(privateKey)
	}
	return gcsSignedDocumentImageURLIssuer{
		bucket:         bucket,
		googleAccessID: googleAccessID,
		privateKey:     key,
		ttl:            ttl,
	}
}

// IssueDocumentImageURL creates a signed GET URL with expiration for downloading a document image from Cloud Storage.
func (i gcsSignedDocumentImageURLIssuer) IssueDocumentImageURL(ctx context.Context, workspaceID, documentID, filename string) (repository.SignedURL, error) {
	expiresAt := time.Now().Add(i.ttl)
	opts := &gcs.SignedURLOptions{
		GoogleAccessID: i.googleAccessID,
		Method:         "GET",
		Expires:        expiresAt,
		Scheme:         gcs.SigningSchemeV4,
	}
	if len(i.privateKey) > 0 {
		opts.PrivateKey = i.privateKey
	} else {
		opts.SignBytes = i.signBytesWithIAM(ctx)
	}
	objectName := storage.SourceObjectName(documentID, filename)
	url, err := gcs.SignedURL(i.bucket, workspaceID+"/"+objectName, opts)
	if err != nil {
		return repository.SignedURL{}, err
	}
	return repository.SignedURL{URL: url, ExpiresAt: expiresAt}, nil
}

// signBytesWithIAM mirrors the upload issuer's IAM signing path for workload
// identity (no local private key).
func (i gcsSignedDocumentImageURLIssuer) signBytesWithIAM(ctx context.Context) func([]byte) ([]byte, error) {
	return func(payload []byte) ([]byte, error) {
		if i.googleAccessID == "" {
			return nil, fmt.Errorf("GCS_SIGNING_SERVICE_ACCOUNT_EMAIL is required for signed image URLs")
		}
		svc, err := iamcredentials.NewService(ctx)
		if err != nil {
			return nil, fmt.Errorf("create iam credentials service: %w", err)
		}
		resp, err := svc.Projects.ServiceAccounts.SignBlob(
			"projects/-/serviceAccounts/"+i.googleAccessID,
			&iamcredentials.SignBlobRequest{Payload: base64.StdEncoding.EncodeToString(payload)},
		).Do()
		if err != nil {
			return nil, fmt.Errorf("sign image URL bytes: %w", err)
		}
		signed, err := base64.StdEncoding.DecodeString(resp.SignedBlob)
		if err != nil {
			return nil, fmt.Errorf("decode signed image URL bytes: %w", err)
		}
		return signed, nil
	}
}

type fakeGCSDocumentUploadURLIssuer struct {
	base   string
	bucket string
}

// NewFakeGCSDocumentUploadURLIssuer constructs an upload URL issuer targeting local fake-GCS.
func NewFakeGCSDocumentUploadURLIssuer(base, bucket string) repository.DocumentUploadURLIssuer {
	return fakeGCSDocumentUploadURLIssuer{base: base, bucket: bucket}
}

// IssueDocumentUploadURL returns upload targets for local fake-GCS testing.
func (i fakeGCSDocumentUploadURLIssuer) IssueDocumentUploadURL(ctx context.Context, workspaceID, objectName, contentType string, fileSize int64) (repository.DocumentUploadTarget, error) {
	// fake-gcs does not enforce Content-Length on POST uploads; the parameter is
	// accepted to satisfy the interface and kept identical to the GCS branch so
	// integration tests exercise the same call shape.
	_ = fileSize
	return repository.DocumentUploadTarget{
		URL:         storage.BuildDocumentUploadURL(i.base, i.bucket, workspaceID, objectName),
		Method:      "POST",
		ContentType: contentType,
	}, nil
}

type gcsSignedDocumentUploadURLIssuer struct {
	bucket         string
	googleAccessID string
	privateKey     []byte
	ttl            time.Duration
}

// NewGCSSignedDocumentUploadURLIssuer creates an upload URL issuer using V4 signed Cloud Storage PUT URLs.
func NewGCSSignedDocumentUploadURLIssuer(bucket, googleAccessID, privateKey string, ttl time.Duration) repository.DocumentUploadURLIssuer {
	privateKey = strings.ReplaceAll(privateKey, `\n`, "\n")
	var key []byte
	if privateKey != "" {
		key = []byte(privateKey)
	}
	return gcsSignedDocumentUploadURLIssuer{
		bucket:         bucket,
		googleAccessID: googleAccessID,
		privateKey:     key,
		ttl:            ttl,
	}
}

// IssueDocumentUploadURL creates a signed PUT upload URL bound to the declared file size and content type.
func (i gcsSignedDocumentUploadURLIssuer) IssueDocumentUploadURL(ctx context.Context, workspaceID, objectName, contentType string, fileSize int64) (repository.DocumentUploadTarget, error) {
	expiresAt := time.Now().Add(i.ttl)
	opts := &gcs.SignedURLOptions{
		GoogleAccessID: i.googleAccessID,
		Method:         "PUT",
		Expires:        expiresAt,
		ContentType:    contentType,
		Scheme:         gcs.SigningSchemeV4,
	}
	// Bind the declared file size into the signature via the Content-Length
	// header so a client cannot reuse the URL to upload something larger than
	// what was reserved against the account's quota. V4 signing requires the
	// header to appear verbatim on the PUT for the signature to validate.
	// fileSize is validated > 0 upstream (validateUploadSize), so the binding
	// is always applied.
	opts.Headers = []string{"Content-Length:" + strconv.FormatInt(fileSize, 10)}
	if len(i.privateKey) > 0 {
		opts.PrivateKey = i.privateKey
	} else {
		opts.SignBytes = i.signBytesWithIAM(ctx)
	}
	url, err := gcs.SignedURL(i.bucket, workspaceID+"/"+objectName, opts)
	if err != nil {
		return repository.DocumentUploadTarget{}, err
	}
	return repository.DocumentUploadTarget{
		URL:         url,
		Method:      "PUT",
		ContentType: contentType,
		ExpiresAt:   expiresAt,
	}, nil
}

// signBytesWithIAM uses the IAM Credentials API to sign byte blobs when running under Workload Identity.
func (i gcsSignedDocumentUploadURLIssuer) signBytesWithIAM(ctx context.Context) func([]byte) ([]byte, error) {
	return func(payload []byte) ([]byte, error) {
		if i.googleAccessID == "" {
			return nil, fmt.Errorf("GCS_SIGNING_SERVICE_ACCOUNT_EMAIL is required for signed upload URLs")
		}
		svc, err := iamcredentials.NewService(ctx)
		if err != nil {
			return nil, fmt.Errorf("create iam credentials service: %w", err)
		}
		resp, err := svc.Projects.ServiceAccounts.SignBlob(
			"projects/-/serviceAccounts/"+i.googleAccessID,
			&iamcredentials.SignBlobRequest{Payload: base64.StdEncoding.EncodeToString(payload)},
		).Do()
		if err != nil {
			return nil, fmt.Errorf("sign upload URL bytes: %w", err)
		}
		signed, err := base64.StdEncoding.DecodeString(resp.SignedBlob)
		if err != nil {
			return nil, fmt.Errorf("decode signed upload URL bytes: %w", err)
		}
		return signed, nil
	}
}

// uploadSignedURLTTL reads the signed URL TTL from environment or returns the 15-minute default.
func uploadSignedURLTTL() time.Duration {
	minutes, err := strconv.Atoi(getenvDefault("GCS_SIGNED_URL_TTL_MINUTES", "15"))
	if err != nil || minutes <= 0 {
		return 15 * time.Minute
	}
	return time.Duration(minutes) * time.Minute
}

// getenvDefault retrieves an environment variable or returns the fallback value when empty.
func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
