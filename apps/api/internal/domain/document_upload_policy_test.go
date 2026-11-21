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

func TestCheckUploadCount(t *testing.T) {
	limited := &Account{MaxUploadsPerFiveH: 20, MaxUploadsPerWeek: 100}
	tests := []struct {
		name        string
		account     *Account
		inFiveHours int64
		inWeek      int64
		wantErr     string // empty when another upload is allowed
	}{
		{name: "below both limits", account: limited, inFiveHours: 19, inWeek: 99},
		{name: "five hour limit reached", account: limited, inFiveHours: 20, inWeek: 20, wantErr: "upload limit reached: at most 20 uploads in 5 hours"},
		{name: "week limit reached", account: limited, inFiveHours: 0, inWeek: 100, wantErr: "upload limit reached: at most 100 uploads in 7 days"},
		{name: "both limits reached", account: limited, inFiveHours: 25, inWeek: 120, wantErr: "upload limit reached: at most 20 uploads in 5 hours"},
		{name: "zero limits mean no limit", account: &Account{}, inFiveHours: 1000, inWeek: 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckUploadCount(tt.account, tt.inFiveHours, tt.inWeek)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrUploadLimitReached)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}
