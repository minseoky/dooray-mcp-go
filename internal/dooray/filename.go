package dooray

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	encodedFileNamePattern = regexp.MustCompile(`(?i)filename\*=UTF-8''([^;]+)`)
	plainFileNamePattern   = regexp.MustCompile(`(?i)filename=(?:"([^"]+)"|([^;\s]+))`)
	unsafeFileNameChars    = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1F]`)
)

// contentDispositionFileName extracts the file name from a Content-Disposition
// header, preferring the RFC 5987 encoded form.
func contentDispositionFileName(header string) string {
	if header == "" {
		return ""
	}

	if match := encodedFileNamePattern.FindStringSubmatch(header); match != nil {
		decoded, err := url.QueryUnescape(match[1])
		if err == nil {
			return decoded
		}
		return match[1]
	}

	if match := plainFileNamePattern.FindStringSubmatch(header); match != nil {
		if match[1] != "" {
			return match[1]
		}
		return match[2]
	}

	return ""
}

// sanitizeFileName strips directory components and characters that are illegal
// on Windows or macOS file systems.
func sanitizeFileName(fileName string) string {
	baseName := fileName
	// The header may carry a POSIX or Windows path, so both separators are
	// trimmed regardless of the host platform.
	if index := strings.LastIndexAny(baseName, `/\`); index >= 0 {
		baseName = baseName[index+1:]
	}
	baseName = filepath.Base(baseName)
	baseName = unsafeFileNameChars.ReplaceAllString(baseName, "_")
	baseName = strings.Trim(baseName, " .")

	if baseName == "" || baseName == "_" {
		return "dooray-attachment"
	}
	return baseName
}

// EscapeComponent percent-encodes a single URL path component the way
// JavaScript's encodeURIComponent does.
func EscapeComponent(value string) string {
	var builder strings.Builder
	for _, b := range []byte(value) {
		if isUnreservedComponentByte(b) {
			builder.WriteByte(b)
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(upperHex[b>>4])
		builder.WriteByte(upperHex[b&0x0F])
	}
	return builder.String()
}

const upperHex = "0123456789ABCDEF"

func isUnreservedComponentByte(b byte) bool {
	switch {
	case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9':
		return true
	}
	return strings.IndexByte("-_.!~*'()", b) >= 0
}
