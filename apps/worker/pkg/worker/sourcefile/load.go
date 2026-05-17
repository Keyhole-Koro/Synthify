package sourcefile

import (
	"context"
	"fmt"

	"github.com/synthify/backend/packages/shared/domain"
	"github.com/synthify/backend/packages/shared/storage"
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

// Load fills file.Content from the gcsfuse mount. The uploads bucket is
// mounted read-only into the worker, so source files are read as local files
// keyed by {WorkspaceID}/{DocumentID}. There is no HTTP fallback: a missing
// mount or a missing object is a hard error.
func Load(ctx context.Context, fs *storage.FileSystem, file *domain.SourceFile) error {
	if file == nil {
		return fmt.Errorf("source file is nil")
	}
	if len(file.Content) > 0 {
		return nil
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
