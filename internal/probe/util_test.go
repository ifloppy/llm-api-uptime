package probe

import (
	"bytes"
	"testing"
)

func TestStripBOM(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "nil input returns nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty input returns empty",
			input: []byte{},
			want:  []byte{},
		},
		{
			name:  "short input no BOM",
			input: []byte{0xEF, 0xBB},
			want:  []byte{0xEF, 0xBB},
		},
		{
			name:  "BOM only returns empty",
			input: []byte{0xEF, 0xBB, 0xBF},
			want:  []byte{},
		},
		{
			name:  "BOM with JSON content",
			input: append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"ok":true}`)...),
			want:  []byte(`{"ok":true}`),
		},
		{
			name:  "no BOM passes through unchanged",
			input: []byte(`{"ok":true}`),
			want:  []byte(`{"ok":true}`),
		},
		{
			name:  "3-byte input that looks like BOM but is not",
			input: []byte{0xEF, 0xBB, 0xC0},
			want:  []byte{0xEF, 0xBB, 0xC0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripBOM(tt.input)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("stripBOM(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
