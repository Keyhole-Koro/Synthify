// Package imageref governs how images flow into LLM-authored HTML output.
//
// The LLM must never write image URLs itself: it hallucinates filenames,
// invents domains, and corrupts paths. Instead the worker registers each
// usable image here under a stable integer id, hands the LLM only the
// placeholder tokens (see PromptHint), and rewrites those tokens into
// <img data-file-id="..."> markers after generation (see Render). Any token
// the model emits that is not in the registry is dropped, so a hallucinated
// reference can never reach the saved HTML.
//
// The marker carries the file id, NOT a URL: a real image URL is a short-lived
// signed URL minted at display time, so baking one into the persisted HTML
// would leave a dead link once it expired (and would expose the object without
// auth). The web client resolves data-file-id to a fresh signed URL when it
// renders the content.
package imageref

import (
	"fmt"
	"sort"
	"sync"
)

// Image is a single registered image the LLM may reference by its id.
//
// FileID identifies the stored document file; the web client exchanges it for
// a short-lived signed URL at display time. Alt is human-readable description
// text used both in the prompt hint and the rendered <img alt>.
type Image struct {
	FileID string
	Alt    string
}

// Registry holds the images available to a single job. It is reset per job via
// (*base.Context).BeginJob, so ids restart at 1 for every document and never
// leak across jobs. It is safe for concurrent use because tools within a job
// may run in parallel.
type Registry struct {
	mu     sync.Mutex
	images map[int]Image
	next   int
}

// NewRegistry returns an empty registry with ids starting at 1.
func NewRegistry() *Registry {
	return &Registry{images: map[int]Image{}, next: 1}
}

// Reset clears all registered images and rewinds ids back to 1. Called at the
// start of each job so one document's images never bleed into the next.
func (r *Registry) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.images = map[int]Image{}
	r.next = 1
}

// Register adds an image and returns its id. A blank fileID is rejected with id
// 0 and ok=false, so callers never register a reference that cannot resolve to
// an image at display time.
func (r *Registry) Register(fileID, alt string) (id int, ok bool) {
	if r == nil || fileID == "" {
		return 0, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id = r.next
	r.next++
	r.images[id] = Image{FileID: fileID, Alt: alt}
	return id, true
}

// Lookup returns the image registered under id, if any.
func (r *Registry) Lookup(id int) (Image, bool) {
	if r == nil {
		return Image{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	img, ok := r.images[id]
	return img, ok
}

// Empty reports whether no images are registered. Callers use this to skip the
// prompt hint entirely when there is nothing for the LLM to reference.
func (r *Registry) Empty() bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.images) == 0
}

// PromptHint returns the instruction block to append to an HTML-generation
// system prompt, listing every available image by token. It returns "" when
// the registry is empty so the prompt stays clean when there are no images.
//
// The tokens use the [[img:N]] form that Render rewrites; the prompt forbids
// the model from writing <img> tags or URLs directly.
func (r *Registry) PromptHint() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	ids := make([]int, 0, len(r.images))
	for id := range r.images {
		ids = append(ids, id)
	}
	imgs := r.images
	r.mu.Unlock()
	if len(ids) == 0 {
		return ""
	}
	sort.Ints(ids)

	var b []byte
	b = append(b, "\n\nAVAILABLE IMAGES: To place an image, write ONLY its token (e.g. [[img:1]]) on its own. "...)
	b = append(b, "Never write an <img> tag, a URL, or a src attribute yourself — the token is rewritten into the real image. "...)
	b = append(b, "Use a token only if it genuinely belongs in the content; do not invent tokens for images not listed.\n"...)
	for _, id := range ids {
		alt := imgs[id].Alt
		if alt == "" {
			alt = "(no description)"
		}
		b = append(b, fmt.Sprintf("  - [[img:%d]]: %s\n", id, alt)...)
	}
	return string(b)
}
