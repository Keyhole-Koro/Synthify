package sourceio

import (
	"context"
	"fmt"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	storage "github.com/synthify/backend/apps/worker/pkg/worker/storage"
)

// LoadAll resolves the content of every source file from the gcsfuse mount,
// returning a copy with Content populated. It fails on the first file that
// cannot be read; partial results are never returned.
func LoadAll(ctx context.Context, fs *storage.FileSystem, files []domain.SourceFile) ([]domain.SourceFile, error) {
	out := make([]domain.SourceFile, len(files))
	copy(out, files)
	for i := range out {
		if err := Load(ctx, fs, &out[i]); err != nil {
			return nil, fmt.Errorf("load[%d] %s: %w", i, out[i].Filename, err)
		}
	}
	return out, nil
}

// Load fills file.Content. Sources resolve one of two ways, dispatched on the
// URI scheme:
//
//   - An http(s):// URI is an external image (path B) downloaded over HTTP,
//     bounded and restricted to image/* — see fetchHTTP.
//   - Anything else is a mounted document (path A) read from the read-only
//     gcsfuse uploads bucket, keyed by {WorkspaceID}/{DocumentID}. A missing
//     mount or object is a hard error.
func Load(ctx context.Context, fs *storage.FileSystem, file *domain.SourceFile) error {
	if file == nil {
		return fmt.Errorf("source file is nil")
	}
	if len(file.Content) > 0 {
		return nil
	}
	if isHTTPURI(file.URI) {
		return fetchHTTP(ctx, file)
	}
	if fs == nil {
		return fmt.Errorf("gcsfuse mount is not configured")
	}

	ok, err := fs.PopulateSourceFile(file)
	if err != nil {
		return fmt.Errorf("read %s/%s from mount: %w", file.WorkspaceID, file.DocumentID, err)
	}
	if !ok {
		return fmt.Errorf("source file not found on mount: %s/%s", file.WorkspaceID, file.DocumentID)
	}
	return nil
}
