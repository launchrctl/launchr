package launchr

import (
	"bytes"
	"io"
	"sync"
)

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
	// Check if we should flush based on content
	shouldFlush := m.shouldFlush(p)

	// If we should flush AND there's no potential sensitive data at the end, flush
	if shouldFlush && !m.hasPotentialSensitiveData() {
		if _, writeErr := m.w.Write(m.buf.Bytes()); writeErr != nil {
			return 0, writeErr
		}
		// Reset the buffer after writing.
		m.buf.Reset()
	}

	return len(p), nil
}

// shouldFlush determines if we should flush based on the content
func (m *MaskingWriter) shouldFlush(p []byte) bool {
	// Flush on newlines (most common for terminal output)
	if bytes.Contains(p, []byte{'\n'}) {
		return true
	}

	// Flush on other natural boundaries
	if bytes.Contains(p, []byte{'\r'}) || bytes.Contains(p, []byte{'\t'}) {
		return true
	}

	// Flush if buffer is getting large (safety valve)
	if m.buf.Len() > 4096 {
		return true
	}

	return false
}

// hasPotentialSensitiveData checks if buffer might contain partial sensitive data
func (m *MaskingWriter) hasPotentialSensitiveData() bool {
	if m.mask == nil || len(m.mask.strings) == 0 {
		return false
	}

	bufData := m.buf.Bytes()
	bufLen := len(bufData)

	// Check if any sensitive string could be partially present at the end
	for _, sensitive := range m.mask.strings {
		sensitiveLen := len(sensitive)
		if sensitiveLen <= 1 {
			continue // Skip very short patterns
		}

		// Check if any prefix of the sensitive string matches the end of our buffer
		maxCheck := sensitiveLen - 1
		if maxCheck > bufLen {
			maxCheck = bufLen
		}

		for i := 1; i <= maxCheck; i++ {
			if bytes.HasSuffix(bufData, sensitive[:i]) {
				return true
			}
		}
	}

	return false
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
	mx      sync.Mutex
	strings [][]byte
	mask    []byte
}

// ServiceInfo implements [Service] interface.
func (p *SensitiveMask) ServiceInfo() ServiceInfo {
	return ServiceInfo{}
}

// ServiceCreate implements [ServiceCreate] interface.
func (p *SensitiveMask) ServiceCreate(_ *ServiceManager) Service {
	return NewSensitiveMask("****")
}

// MaskWriter returns a wrapped writer with masked output.
func (p *SensitiveMask) MaskWriter(w io.Writer) io.WriteCloser {
	return NewMaskingWriter(w, p)
}

// Clone creates a copy of a sensitive mask.
func (p *SensitiveMask) Clone() *SensitiveMask {
	p.mx.Lock()
	defer p.mx.Unlock()

	// Create a new slice with the same length and capacity
	clonedStrings := make([][]byte, len(p.strings))

	// Deep copy each []byte slice
	for i, str := range p.strings {
		if str != nil {
			clonedStrings[i] = make([]byte, len(str))
			copy(clonedStrings[i], str)
		}
	}

	// Clone the mask slice as well
	var clonedMask []byte
	if p.mask != nil {
		clonedMask = make([]byte, len(p.mask))
		copy(clonedMask, p.mask)
	}

	return &SensitiveMask{
		strings: clonedStrings,
		mask:    clonedMask,
	}
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

	if p == nil || len(p.strings) == 0 {
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
	p.mx.Lock()
	defer p.mx.Unlock()
	p.strings = append(p.strings, []byte(s))
}

// NewSensitiveMask creates a sensitive mask replacing strings with mask value.
func NewSensitiveMask(mask string) *SensitiveMask {
	return &SensitiveMask{
		mask: []byte(mask),
	}
}
