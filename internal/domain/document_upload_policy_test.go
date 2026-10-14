package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateDocumentUploadTypeAllowsNonExecutableFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		mimeType string
	}{
		{name: "pdf", filename: "paper.pdf", mimeType: "application/pdf"},
		{name: "text without extension", filename: "notes", mimeType: "text/plain; charset=utf-8"},
		{name: "markdown with octet stream", filename: "README.md", mimeType: "application/octet-stream"},
		{name: "source code without mime", filename: "main.go", mimeType: ""},
		{name: "unknown extension with octet stream", filename: "payload.dat", mimeType: "application/octet-stream"},
		{name: "unknown no mime", filename: "blob", mimeType: ""},
		{name: "image", filename: "diagram.png", mimeType: "image/png"},
		{name: "zip", filename: "project.zip", mimeType: "application/zip"},
		{name: "docx", filename: "proposal.docx", mimeType: "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, ValidateDocumentUploadType(tt.filename, tt.mimeType))
		})
	}
}

func TestValidateDocumentUploadTypeRejectsExecutableFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		mimeType string
	}{
		{name: "windows exe", filename: "installer.exe", mimeType: "application/x-msdownload"},
		{name: "dll by extension", filename: "plugin.dll", mimeType: "text/plain"},
		{name: "executable by mime", filename: "payload.dat", mimeType: "application/x-executable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorIs(t, ValidateDocumentUploadType(tt.filename, tt.mimeType), ErrUnsupportedDocumentType)
		})
	}
}
