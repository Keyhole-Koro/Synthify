package storage

import (
	"encoding/json"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

// FileSystem manages access to files via a local mount (e.g., GCS FUSE).
type FileSystem struct {
	MountPath string
}

// NewFileSystem creates a new FileSystem with the given mount path.
func NewFileSystem(mountPath string) *FileSystem {
	return &FileSystem{MountPath: mountPath}
}

// Document layout under the mount. {ws}/{doc}/ is ALWAYS a directory with two
// role-separated subtrees, so the path can never collide with a same-named file
// (the bug that made every extract fail). See
// docs/improvements/gcs-storage-layout-redesign.md.
//
//	{ws}/{doc}/source/     uploaded original (API writes, worker reads)
//	{ws}/{doc}/extracted/  worker's working tree (single file copy / zip expansion)
const (
	sourceSubdir    = "source"
	extractedSubdir = "extracted"
)

// DocPath returns the document's root directory within the mount: {ws}/{doc}/.
// Returns an empty string if MountPath is not set or IDs are missing.
func (fs *FileSystem) DocPath(workspaceID, documentID string) string {
	if fs.MountPath == "" || workspaceID == "" || documentID == "" {
		return ""
	}
	return filepath.Join(fs.MountPath, workspaceID, documentID)
}

// SourceDir returns {ws}/{doc}/source/ — where the uploaded original lives.
func (fs *FileSystem) SourceDir(workspaceID, documentID string) string {
	root := fs.DocPath(workspaceID, documentID)
	if root == "" {
		return ""
	}
	return filepath.Join(root, sourceSubdir)
}

// ExtractedDir returns {ws}/{doc}/extracted/ — the worker's working tree.
func (fs *FileSystem) ExtractedDir(workspaceID, documentID string) string {
	root := fs.DocPath(workspaceID, documentID)
	if root == "" {
		return ""
	}
	return filepath.Join(root, extractedSubdir)
}

// sourceFilePath returns the single original file under source/. The uploader
// writes exactly one object there (original.{ext} or archive.zip), so we read
// whichever file is present rather than depending on its exact name.
func (fs *FileSystem) sourceFilePath(workspaceID, documentID string) (string, error) {
	dir := fs.SourceDir(workspaceID, documentID)
	if dir == "" {
		return "", nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", nil
}

// Open attempts to open the uploaded original under source/.
func (fs *FileSystem) Open(workspaceID, documentID string) (io.ReadCloser, error) {
	path, err := fs.sourceFilePath(workspaceID, documentID)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	return os.Open(path)
}

// ReadAll reads the uploaded original under source/. Returns nil content when
// nothing is present yet (the upload may still be in flight).
func (fs *FileSystem) ReadAll(workspaceID, documentID string) ([]byte, error) {
	path, err := fs.sourceFilePath(workspaceID, documentID)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	return os.ReadFile(path)
}

// PopulateSourceFile fills Content and MimeType of a SourceFile from the
// original under source/. It also adopts the on-disk filename so MIME sniffing
// by extension works even when the caller only knew the document_id.
func (fs *FileSystem) PopulateSourceFile(file *domain.SourceFile) (bool, error) {
	if file == nil {
		return false, nil
	}

	path, err := fs.sourceFilePath(file.WorkspaceID, file.DocumentID)
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	file.Content = body
	if strings.TrimSpace(file.Filename) == "" {
		file.Filename = filepath.Base(path)
	}
	if strings.TrimSpace(file.MimeType) == "" {
		file.MimeType = mime.TypeByExtension(filepath.Ext(file.Filename))
	}
	return true, nil
}

// CachePath returns the path to a cache entry within the mount.
func (fs *FileSystem) CachePath(documentID, category, key string) string {
	if fs.MountPath == "" {
		return ""
	}
	// .cache/v1/{documentID}/{category}/{key}.json
	return filepath.Join(fs.MountPath, ".cache", "v1", documentID, category, key+".json")
}

// ReadCache attempts to read and decode a JSON cache entry from the mount.
func (fs *FileSystem) ReadCache(documentID, category, key string, target any) (bool, error) {
	path := fs.CachePath(documentID, category, key)
	if path == "" {
		return false, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(target); err != nil {
		return false, err
	}
	return true, nil
}

// WriteCache encodes and writes data as a JSON cache entry to the mount.
func (fs *FileSystem) WriteCache(documentID, category, key string, data any) error {
	path := fs.CachePath(documentID, category, key)
	if path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(f).Encode(data); err != nil {
		f.Close()
		return err
	}
	f.Close()

	return os.Rename(tmpPath, path)
}

// CheckpointPath returns the path to a checkpoint entry within the mount.
func (fs *FileSystem) CheckpointPath(jobID, stage string) string {
	if fs.MountPath == "" || jobID == "" || stage == "" {
		return ""
	}
	// .checkpoints/{jobID}/{stage}.json
	return filepath.Join(fs.MountPath, ".checkpoints", jobID, stage+".json")
}

// ReadCheckpoint attempts to read and decode a CheckpointEnvelope from the mount.
func (fs *FileSystem) ReadCheckpoint(jobID, stage string, target *domain.CheckpointEnvelope) (bool, error) {
	path := fs.CheckpointPath(jobID, stage)
	if path == "" {
		return false, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(target); err != nil {
		return false, err
	}
	return true, nil
}

// WriteCheckpoint encodes and writes a CheckpointEnvelope to the mount.
func (fs *FileSystem) WriteCheckpoint(jobID, stage string, envelope domain.CheckpointEnvelope) error {
	path := fs.CheckpointPath(jobID, stage)
	if path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(f).Encode(envelope); err != nil {
		f.Close()
		return err
	}
	f.Close()

	return os.Rename(tmpPath, path)
}
