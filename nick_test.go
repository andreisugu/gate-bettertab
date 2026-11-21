package bettertab

import (
	"testing"

	"go.minekube.com/gate/pkg/util/uuid"
)

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

func TestStandaloneNicknameLifecycle(t *testing.T) {
	states := NewPlayerStateManager()
	id := uuid.New()

	val, err := setTargetNickname(nil, states, id, "RealSteve", "&bCoolNick", true)
	if err != nil {
		t.Fatalf("setTargetNickname error: %v", err)
	}
	if val != "&bCoolNick" {
		t.Fatalf("got %q, want &bCoolNick", val)
	}
	if got := ResolveNickname(id, nil, states); got != "&bCoolNick" {
		t.Fatalf("ResolveNickname = %q, want &bCoolNick", got)
	}

	cleared, err := clearTargetNickname(states, id)
	if err != nil || !cleared {
		t.Fatalf("clearTargetNickname = (%v, %v), want (true, nil)", cleared, err)
	}
	if got := ResolveNickname(id, nil, states); got != "" {
		t.Fatalf("expected cleared nickname, got %q", got)
	}
}
