package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	gcs "cloud.google.com/go/storage"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/packages/shared/applog"
	"github.com/synthify/backend/packages/shared/config"
	jobstatus "github.com/synthify/backend/packages/shared/job/status"
	"github.com/synthify/backend/packages/shared/repository"
	"github.com/synthify/backend/packages/shared/repository/mock"
	"github.com/synthify/backend/packages/shared/repository/postgres"
	"github.com/synthify/backend/packages/shared/storage"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
)

type AppContext struct {
	Store    Store
	Notifier jobstatus.Notifier
}

func Bootstrap(ctx context.Context, gcsURLBase, firebaseProjectID string, logger applog.Logger, nrApp *newrelic.Application) *AppContext {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	store := InitStore(ctx, NewDocumentUploadURLIssuer(gcsURLBase), logger, nrApp)
	notifier := jobstatus.NewNotifier(ctx, firebaseProjectID, logger)
	return &AppContext{
		Store:    store,
		Notifier: notifier,
	}
}

// Store aggregates every persistence-facing capability via embedded
// domain-specific repository interfaces. Callers should prefer depending
// on a narrower interface from packages/shared/repository when possible.
type Store interface {
	repository.AccountRepository
	repository.WorkspaceRepository
	repository.DocumentRepository
	repository.TreeRepository
	repository.ItemRepository
	repository.UsageRepository
	repository.CheckpointRepository
	repository.JobLogWriter
}

type fakeGCSDocumentUploadURLIssuer struct {
	base string
}

func NewFakeGCSDocumentUploadURLIssuer(base string) repository.DocumentUploadURLIssuer {
	return fakeGCSDocumentUploadURLIssuer{base: base}
}

func (i fakeGCSDocumentUploadURLIssuer) IssueDocumentUploadURL(ctx context.Context, workspaceID, objectName, contentType string) (repository.DocumentUploadTarget, error) {
	return repository.DocumentUploadTarget{
		URL:         storage.BuildDocumentUploadURL(i.base, workspaceID, objectName),
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

func (i gcsSignedDocumentUploadURLIssuer) IssueDocumentUploadURL(ctx context.Context, workspaceID, objectName, contentType string) (repository.DocumentUploadTarget, error) {
	expiresAt := time.Now().Add(i.ttl)
	opts := &gcs.SignedURLOptions{
		GoogleAccessID: i.googleAccessID,
		Method:         "PUT",
		Expires:        expiresAt,
		ContentType:    contentType,
		Scheme:         gcs.SigningSchemeV4,
	}
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

func NewDocumentUploadURLIssuer(base string) repository.DocumentUploadURLIssuer {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GCS_UPLOAD_ISSUER")))
	if mode == "signed" {
		return NewGCSSignedDocumentUploadURLIssuer(
			getenvDefault("GCS_BUCKET", storage.DefaultBucket),
			os.Getenv("GCS_SIGNING_SERVICE_ACCOUNT_EMAIL"),
			os.Getenv("GCS_SIGNING_PRIVATE_KEY"),
			uploadSignedURLTTL(),
		)
	}
	return NewFakeGCSDocumentUploadURLIssuer(base)
}

func uploadSignedURLTTL() time.Duration {
	minutes, err := strconv.Atoi(getenvDefault("GCS_SIGNED_URL_TTL_MINUTES", "15"))
	if err != nil || minutes <= 0 {
		return 15 * time.Minute
	}
	return time.Duration(minutes) * time.Minute
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func NewDocumentSourceURLBuilder(base string) repository.DocumentSourceURLBuilder {
	return func(workspaceID, documentID string) string {
		return storage.BuildDocumentSourceURL(base, workspaceID, documentID)
	}
}

func InitStore(ctx context.Context, uploadURLIssuer repository.DocumentUploadURLIssuer, logger applog.Logger, nrApp *newrelic.Application) Store {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	if dsn := config.LoadStore().DatabaseURL; dsn != "" {
		var lastErr error
		for attempt := 1; attempt <= 10; attempt++ {
			store, err := postgres.NewStore(ctx, dsn, uploadURLIssuer, logger, nrApp)
			if err == nil {
				logger.Info(ctx, "app.store_initialized", map[string]any{"type": "postgres"})
				return store
			}
			lastErr = err
			logger.Warn(ctx, "app.store_init_retry", err, map[string]any{"attempt": attempt})
			time.Sleep(2 * time.Second)
		}
		logger.Error(ctx, "app.store_init_failed", lastErr, nil)
		panic(lastErr)
	}
	logger.Info(ctx, "app.store_initialized", map[string]any{"type": "mock"})
	return mock.NewStore()
}
