package sourceio

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

// maxFetchBytes caps an external image download so a hostile or mistaken URL
// cannot exhaust worker memory. Images that legitimately exceed this are not
// supported as inline output.
const maxFetchBytes = 20 << 20 // 20 MiB

// fetchClient is the HTTP client used for external image fetches. A fresh
// client with no special transport keeps redirects on by default; the size cap
// is enforced on the body, not via Content-Length, so a lying server is still
// bounded.
var fetchClient = &http.Client{}

// fetchHTTP populates file.Content by downloading file.URI over HTTP(S). It is
// the path-B counterpart to the gcsfuse mount read in Load: external images the
// LLM may embed in output, addressed by absolute URL rather than by
// {WorkspaceID}/{DocumentID}.
//
// Only image/* content is accepted — this path exists for inline images, and
// refusing other types keeps it from becoming a general SSRF fetch primitive.
// The body is hard-capped at maxFetchBytes regardless of the server's headers.
func fetchHTTP(ctx context.Context, file *domain.SourceFile) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.URI, nil)
	if err != nil {
		return fmt.Errorf("build fetch request for %s: %w", file.URI, err)
	}
	resp, err := fetchClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", file.URI, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: unexpected status %d", file.URI, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if mt := mediaType(contentType); !strings.HasPrefix(mt, "image/") {
		return fmt.Errorf("fetch %s: unsupported content type %q (only image/* is allowed)", file.URI, contentType)
	}

	// LimitReader at cap+1 so we can distinguish "exactly at cap" from "over".
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes+1))
	if err != nil {
		return fmt.Errorf("read body for %s: %w", file.URI, err)
	}
	if len(body) > maxFetchBytes {
		return fmt.Errorf("fetch %s: image exceeds %d byte limit", file.URI, maxFetchBytes)
	}

	file.Content = body
	if file.MimeType == "" {
		file.MimeType = mediaType(contentType)
	}
	return nil
}

// mediaType strips any "; charset=..." parameters from a Content-Type header.
func mediaType(contentType string) string {
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	return strings.ToLower(strings.TrimSpace(contentType))
}

// isHTTPURI reports whether uri is an absolute http(s) URL, i.e. a path-B
// external source rather than a mounted document.
func isHTTPURI(uri string) bool {
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}
