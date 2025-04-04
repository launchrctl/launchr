package launchr

import (
	"bytes"
	"io"
)

var globalSensitiveMask *SensitiveMask

func init() {
	globalSensitiveMask = NewSensitiveMask("****")
}

// GlobalSensitiveMask returns global app sensitive mask.
func GlobalSensitiveMask() *SensitiveMask {
	return globalSensitiveMask
}

// MaskingWriter is a writer that masks sensitive data in the input stream.
// It buffers data to handle cases where sensitive data spans across writes.
type MaskingWriter struct {
	w    io.Writer
	mask *SensitiveMask
	buf  bytes.Buffer
}

// NewMaskingWriter initializes a new MaskingWriter.
func NewMaskingWriter(wrappedWriter io.Writer, mask *SensitiveMask) io.WriteCloser {
	return &MaskingWriter{
		w:    wrappedWriter,
		mask: mask,
		buf:  bytes.Buffer{},
	}
}

// Write applies masking to the input and writes to the wrapped writer.
func (m *MaskingWriter) Write(p []byte) (n int, err error) {
	// Append the new data to the buf.
	m.buf.Write(p)

	// Process the buf and mask the sensitive values.
	data := m.buf.Bytes()
	masked, lastOrigEnd, lastMatchEnd := m.mask.ReplaceAll(data)

	// Write the fully masked content up to the last complete match only.
	// Keep any leftover (incomplete) data in the buf.
	if lastMatchEnd >= 0 {
		remaining := data[lastOrigEnd:]
		m.buf.Reset()
		m.buf.Write(remaining)

		processed := masked[:lastMatchEnd]
		// Write the processed portion.
		if _, writeErr := m.w.Write(processed); writeErr != nil {
			return 0, writeErr
		}
	}

	// If no complete sensitive data was found, keep everything in the buf.
	// Write the buffer periodically if the input slice `p` is less than its capacity.
	if len(p) < cap(p) && m.buf.Len() > 0 {
		// Write all remaining buffer content after masking.
		if _, writeErr := m.w.Write(m.buf.Bytes()); writeErr != nil {
			return 0, writeErr
		}
		// Reset the buffer after writing.
		m.buf.Reset()
	}

	return len(p), nil
}

// Close flushes any remaining data in the buf.
func (m *MaskingWriter) Close() error {
	if m.buf.Len() > 0 {
		// Write the remainder of the buf after masking.
		masked, _, _ := m.mask.ReplaceAll(m.buf.Bytes())
		if _, err := m.w.Write(masked); err != nil {
			return err
		}
		m.buf.Reset()
	}
	if w, ok := m.w.(io.Closer); ok {
		return w.Close()
	}
	return nil
}

// SensitiveMask replaces sensitive strings with a mask.
type SensitiveMask struct {
	strings [][]byte
	mask    []byte
}

// String implements [fmt.Stringer] to occasionally not render sensitive data.
func (p *SensitiveMask) String() string { return "" }

// ReplaceAll replaces sensitive strings in the given bytes b.
// It returns the modified string and last index of replaced parts to track where the last change was made
// for before change and after change bytes.
func (p *SensitiveMask) ReplaceAll(b []byte) (resultBytes []byte, lastBefore, lastAfter int) {
	// Create a buffer to build the result.
	var result bytes.Buffer
	start := 0

	// Initialize tracking variables
	lastBefore = -1
	lastAfter = -1

	if len(p.strings) == 0 {
		return b, lastBefore, lastAfter
	}

	for start < len(b) {
		earliestMatchIndex := -1
		matchLength := 0

		// Look for all substrings and find the earliest occurrence.
		for _, s := range p.strings {
			if idx := bytes.Index(b[start:], s); idx != -1 {
				// If this is the earliest match so far, update earliestMatchIndex and matchLength.
				absoluteIdx := start + idx
				if earliestMatchIndex == -1 || absoluteIdx < earliestMatchIndex {
					earliestMatchIndex = absoluteIdx
					matchLength = len(s)
				}
			}
		}

		// If a match was found, replace it with the mask.
		if earliestMatchIndex != -1 {
			// Update lastBefore to track the index before replacing.
			lastBefore = earliestMatchIndex + matchLength

			// Write everything up to the match.
			result.Write(b[start:earliestMatchIndex])
			// Write the mask instead of the matched string.
			result.Write(p.mask)

			// Update lastAfter to track the index after replacing.
			lastAfter = result.Len()

			// Move the start index past the matched string.
			start = earliestMatchIndex + matchLength
		} else {
			// If no matches are found, append the rest of the buffer and break.
			result.Write(b[start:])
			break
		}
	}

	// Return the final result and the tracked indices.
	return result.Bytes(), lastBefore, lastAfter
}

// AddString adds a string to mask.
func (p *SensitiveMask) AddString(s string) {
	p.strings = append(p.strings, []byte(s))
}

// NewSensitiveMask creates a sensitive mask replacing strings with mask value.
func NewSensitiveMask(mask string, strings ...string) *SensitiveMask {
	bytestrings := make([][]byte, len(strings))
	for i := 0; i < len(strings); i++ {
		bytestrings[i] = []byte(strings[i])
	}
	return &SensitiveMask{
		mask:    []byte(mask),
		strings: bytestrings,
	}
}
