package launchr

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MaskingWriter(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name   string
		chunks [][]byte       // Stream parts to simulate multiple writes
		mask   *SensitiveMask // Mask replacement
		exp    string         // Expected output after masking
	}
	mask := NewSensitiveMask("****")
	mask.AddString("987-65-4321")
	mask.AddString("123-45-6789")
	mask.AddString("\"\\escaped\nnewline")
	tests := []testCase{
		{
			name:   "Empty mask",
			chunks: [][]byte{[]byte("This is a clean stream with no sensitive data.")},
			mask:   NewSensitiveMask("****"),
			exp:    "This is a clean stream with no sensitive data.",
		},
		{
			name:   "No values to mask",
			chunks: [][]byte{[]byte("This is a clean stream with no sensitive data.")},
			mask:   mask,
			exp:    "This is a clean stream with no sensitive data.",
		},
		{
			name:   "Two values to mask",
			chunks: [][]byte{[]byte("Sensitive data: \"\\escaped\nnewline, 123-45-6789 and also 987-65-4321.")},
			mask:   mask,
			exp:    "Sensitive data: ****, **** and also ****.",
		},
		{
			name: "Sensitive value split across writes",
			chunks: [][]byte{
				[]byte("Sensitive data: 123-45"),
				[]byte("-6789 and 987-65-4321 appears split."),
				[]byte("\"\\escaped\nnewline"),
				[]byte("This is a clean stream with no sensitive data."),
			},
			mask: mask,
			exp:  "Sensitive data: **** and **** appears split.****This is a clean stream with no sensitive data.",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Set up the buffer to capture output
			out := &bytes.Buffer{}

			// Create the MaskingWriter wrapping the out
			mwriter := tt.mask.MaskWriter(out)

			// Simulate multiple writes
			for _, part := range tt.chunks {
				_, err := mwriter.Write(part)
				require.NoError(t, err)
			}

			// Flush any remaining data in the buffer
			err := mwriter.Close()
			require.NoError(t, err)

			// Validate the final output
			assert.Equal(t, tt.exp, out.String())
		})
	}

}
