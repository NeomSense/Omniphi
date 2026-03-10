package keeper

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// ============================================================================
// Canonical Hash Reference Implementation
//
// These are pure functions (no keeper state) that serve as the reference
// specification for content normalization. Clients compute canonical hashes
// off-chain using the same algorithm. The chain stores only the resulting
// hash + spec version. These functions are used by tests as test vectors.
//
// Spec Version 1 Rules:
//   - Text/Docs: strip markdown formatting, normalize whitespace, lowercase, LF line endings
//   - Code: strip comments (// and /* */), collapse whitespace, sort import lines, trim blanks
//   - Dataset: sort JSON lines lexicographically, normalize whitespace
// ============================================================================

// CanonicalizeText normalizes text/documentation content.
// Rules:
//  1. Convert to lowercase
//  2. Replace \r\n and \r with \n (normalize line endings)
//  3. Strip basic markdown formatting (**, __, *, _)
//  4. Collapse consecutive whitespace within lines to single space
//  5. Trim leading/trailing whitespace per line
//  6. Remove empty lines
//  7. Ensure trailing newline
func CanonicalizeText(raw []byte) []byte {
	s := string(raw)

	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Lowercase
	s = strings.ToLower(s)

	// Strip markdown bold/italic markers
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	// Single markers after double to avoid double-removal
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "_", " ") // underscore italic → space (then collapsed)

	// Process line by line
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		// Collapse whitespace
		line = collapseWhitespace(line)
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}

	if len(result) == 0 {
		return []byte("\n")
	}

	return []byte(strings.Join(result, "\n") + "\n")
}

// CanonicalizeCode normalizes source code content.
// Rules:
//  1. Normalize line endings to \n
//  2. Strip single-line comments (// ...)
//  3. Strip multi-line comments (/* ... */)
//  4. Sort import/include blocks (lines starting with "import " or "#include ")
//  5. Collapse consecutive whitespace to single space within lines
//  6. Trim leading/trailing whitespace per line
//  7. Remove empty lines
//  8. Ensure trailing newline
func CanonicalizeCode(raw []byte) []byte {
	s := string(raw)

	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Strip multi-line comments /* ... */
	s = stripMultiLineComments(s)

	// Process line by line
	lines := strings.Split(s, "\n")
	var processed []string
	var importBlock []string
	inImportBlock := false

	for _, line := range lines {
		// Strip single-line comments
		line = stripSingleLineComment(line)

		// Collapse whitespace and trim
		line = collapseWhitespace(line)
		line = strings.TrimSpace(line)

		if line == "" {
			// If we were in an import block, flush it
			if inImportBlock && len(importBlock) > 0 {
				sort.Strings(importBlock)
				processed = append(processed, importBlock...)
				importBlock = nil
				inImportBlock = false
			}
			continue
		}

		// Detect import lines
		if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "#include ") {
			inImportBlock = true
			importBlock = append(importBlock, line)
		} else {
			// Flush any pending import block
			if inImportBlock && len(importBlock) > 0 {
				sort.Strings(importBlock)
				processed = append(processed, importBlock...)
				importBlock = nil
				inImportBlock = false
			}
			processed = append(processed, line)
		}
	}

	// Flush trailing import block
	if len(importBlock) > 0 {
		sort.Strings(importBlock)
		processed = append(processed, importBlock...)
	}

	if len(processed) == 0 {
		return []byte("\n")
	}

	return []byte(strings.Join(processed, "\n") + "\n")
}

// CanonicalizeDataset normalizes dataset content (JSON lines or CSV-like).
// Rules:
//  1. Normalize line endings to \n
//  2. Trim whitespace per line
//  3. Remove empty lines
//  4. Sort all lines lexicographically (stable, deterministic order)
//  5. Ensure trailing newline
func CanonicalizeDataset(raw []byte) []byte {
	s := string(raw)

	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	lines := strings.Split(s, "\n")
	var trimmed []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			trimmed = append(trimmed, line)
		}
	}

	sort.Strings(trimmed)

	if len(trimmed) == 0 {
		return []byte("\n")
	}

	return []byte(strings.Join(trimmed, "\n") + "\n")
}

// Canonicalize dispatches to the correct normalizer based on contribution category.
// Supported categories: "docs", "text", "code", "dataset", "data"
func Canonicalize(category string, raw []byte) ([]byte, error) {
	switch strings.ToLower(category) {
	case "docs", "text", "documentation":
		return CanonicalizeText(raw), nil
	case "code", "software", "smart_contract":
		return CanonicalizeCode(raw), nil
	case "dataset", "data":
		return CanonicalizeDataset(raw), nil
	default:
		return nil, fmt.Errorf("unsupported category for canonicalization: %s", category)
	}
}

// ComputeCanonicalHash normalizes content by category, then computes SHA-256.
// Returns the 32-byte hash.
func ComputeCanonicalHash(category string, raw []byte) ([]byte, error) {
	canonical, err := Canonicalize(category, raw)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(canonical)
	return hash[:], nil
}

// ========== Helper Functions ==========

// collapseWhitespace replaces runs of whitespace with a single space.
func collapseWhitespace(s string) string {
	var buf bytes.Buffer
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				buf.WriteRune(' ')
				prevSpace = true
			}
		} else {
			buf.WriteRune(r)
			prevSpace = false
		}
	}
	return buf.String()
}

// stripSingleLineComment removes // comments from a line.
// Handles the case where // appears inside a string literal (simplified: no string tracking).
func stripSingleLineComment(line string) string {
	idx := strings.Index(line, "//")
	if idx < 0 {
		return line
	}
	return line[:idx]
}

// stripMultiLineComments removes all /* ... */ comment blocks from the text.
func stripMultiLineComments(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			// Skip until */
			end := strings.Index(s[i+2:], "*/")
			if end >= 0 {
				i = i + 2 + end + 2
			} else {
				// Unclosed comment — skip rest
				break
			}
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}
