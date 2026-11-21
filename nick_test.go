package bettertab

import "testing"

func TestNormalizeNickname(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		color   bool
		want    string
		wantErr bool
	}{
		{name: "plain", input: "Shadow", want: "Shadow"},
		{name: "underscore", input: "Shadow_01", want: "Shadow_01"},
		{name: "too short", input: "ab", wantErr: true},
		{name: "too long", input: "abcdefghijklmnopq", wantErr: true},
		{name: "space", input: "bad name", wantErr: true},
		{name: "reserved", input: "Admin", wantErr: true},
		{name: "color denied", input: "&bShadow", wantErr: true},
		{name: "color allowed", input: "&BShadow", color: true, want: "&bShadow"},
		{name: "invalid code", input: "&zShadow", color: true, wantErr: true},
		{name: "section sign", input: "§bShadow", color: true, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeNickname(test.input, test.color)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeNickname(%q) = %q, want error", test.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeNickname(%q) error: %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("normalizeNickname(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
