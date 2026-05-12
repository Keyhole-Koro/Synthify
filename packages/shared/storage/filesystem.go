package storage

import (
	"encoding/json"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/synthify/backend/packages/shared/domain"
)

// FileSystem manages access to files via a local mount (e.g., GCS FUSE).
type FileSystem struct {
	MountPath string
}

// NewFileSystem creates a new FileSystem with the given mount path.
func NewFileSystem(mountPath string) *FileSystem {
	return &FileSystem{MountPath: mountPath}
}

// DocPath returns the absolute local path to a document within the mount.
// Returns an empty string if MountPath is not set or IDs are missing.
func (fs *FileSystem) DocPath(workspaceID, documentID string) string {
	if fs.MountPath == "" || workspaceID == "" || documentID == "" {
		return ""
	}
	return filepath.Join(fs.MountPath, workspaceID, documentID)
}

// Open attempts to open a document from the mount.
func (fs *FileSystem) Open(workspaceID, documentID string) (io.ReadCloser, error) {
	path := fs.DocPath(workspaceID, documentID)
	if path == "" {
		return nil, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, nil
	}

	return os.Open(path)
}

// ReadAll attempts to read the entire content of a document from the mount.
func (fs *FileSystem) ReadAll(workspaceID, documentID string) ([]byte, error) {
	path := fs.DocPath(workspaceID, documentID)
	if path == "" {
		return nil, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, nil
	}

	return os.ReadFile(path)
}

// PopulateSourceFile attempts to fill the Content and MimeType of a SourceFile using the mount.
func (fs *FileSystem) PopulateSourceFile(file *domain.SourceFile) (bool, error) {
	if file == nil {
		return false, nil
	}

	body, err := fs.ReadAll(file.WorkspaceID, file.DocumentID)
	if err != nil {
		return false, err
	}
	if body == nil {
		return false, nil
	}

	file.Content = body
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
