package evidence

import (
	"strings"
	"testing"
)

func TestDetectMime(t *testing.T) {
	cases := []struct {
		name    string
		head    []byte
		want    string
		wantErr bool
	}{
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}, "image/jpeg", false},
		{"png", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}, "image/png", false},
		{"gif87a", []byte("GIF87a0101x0"), "image/gif", false},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "image/webp", false},
		{"pdf", []byte("%PDF-1.7 trailing"), "application/pdf", false},
		{"exe pretending to be jpeg", append([]byte{0x4D, 0x5A, 0x90, 0x00}, make([]byte, 12)...), "", true},
		{"text pretending to be pdf", []byte("not a pdf at all, really just text"), "", true},
		{"too short", []byte{0xFF, 0xD8}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectMime(tc.head)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("%s: expected error, got %q", tc.name, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: unexpected error %v", tc.name, err)
			}
			if got != tc.want {
				t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestValidateContentMismatch(t *testing.T) {
	err := ValidateContent("image/jpeg", "image/png")
	if err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("mismatch must be explicit, got %v", err)
	}
	if err := ValidateContent("image/png", "image/png"); err != nil {
		t.Fatalf("matching types must pass: %v", err)
	}
}
