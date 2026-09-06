// Package evidence implements the upload evidence closed loop (PRD P0-2 /
// US-02, execution plan B06): presigned upload, server-side validation
// (size / content-type / magic bytes / hash), and the state machine
// PENDING_UPLOAD → READY | QUARANTINED. Only READY evidence may ever reach
// the AI pipeline (B06 verify).
package evidence

import (
	"bytes"
	"errors"
	"fmt"
)

// DetectMime reports the MIME type implied by the file's magic bytes, or an
// error for unrecognized content. Declared content-type and actual bytes must
// agree — a renamed .exe pretending to be a JPEG must land in QUARANTINED,
// not in the pipeline.
func DetectMime(head []byte) (string, error) {
	if len(head) < 12 {
		return "", errors.New("content too short to identify")
	}
	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", nil
	case bytes.HasPrefix(head, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png", nil
	case bytes.HasPrefix(head, []byte("GIF87a")), bytes.HasPrefix(head, []byte("GIF89a")):
		return "image/gif", nil
	case bytes.HasPrefix(head, []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		return "image/webp", nil
	case bytes.HasPrefix(head, []byte("%PDF-")):
		return "application/pdf", nil
	default:
		return "", fmt.Errorf("unrecognized content (head % x)", head[:12])
	}
}

// ValidateContent is the single gate applied during upload complete.
// declaredType is what the client CLAIMED; detected is what the bytes say.
func ValidateContent(declaredType, detected string) error {
	if declaredType != detected {
		return fmt.Errorf("content-type mismatch: declared %q but bytes are %q", declaredType, detected)
	}
	return nil
}
