package bettertab

import (
	"strings"
	"testing"

	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
)

func extractPlainText(comp c.Component) string {
	var b strings.Builder
	if t, ok := comp.(*c.Text); ok {
		b.WriteString(t.Content)
		for _, child := range t.Extra {
			b.WriteString(extractPlainText(child))
		}
	}
	return b.String()
}

func collectStyledLeaves(comp c.Component) []*c.Text {
	var leaves []*c.Text
	if t, ok := comp.(*c.Text); ok {
		if t.Content != "" {
			leaves = append(leaves, t)
		}
		for _, child := range t.Extra {
			leaves = append(leaves, collectStyledLeaves(child)...)
		}
	}
	return leaves
}

func TestFormatTextLegacyColors(t *testing.T) {
	comp := FormatText("&aGreen &cRed &eYellow &lBold")
	text := extractPlainText(comp)
	if !strings.Contains(text, "Green") || !strings.Contains(text, "Red") {
		t.Errorf("Expected formatted text to contain words, got %q", text)
	}

	bungee := FormatText("&x&f&f&0&0&a&aPink &lBoldPink")
	leaves := collectStyledLeaves(bungee)
	if len(leaves) < 2 || leaves[0].S.Color == nil || leaves[0].S.Color.Hex() != "#ff00aa" {
		t.Fatalf("expected Bungee hex #ff00aa on first leaf, got %+v", leaves)
	}
	if leaves[1].S.Bold != c.True || leaves[1].S.Color == nil || leaves[1].S.Color.Hex() != "#ff00aa" {
		t.Fatalf("expected &l to preserve #ff00aa color, got %+v", leaves[1].S)
	}
}

func TestFormatTextMiniMessage(t *testing.T) {
	comp := FormatText("<green>Online</green> <gold>Store</gold> <bold>VIP</bold>")
	text := extractPlainText(comp)
	if !strings.Contains(text, "Online") || !strings.Contains(text, "VIP") {
		t.Errorf("Expected formatted text to contain words, got %q", text)
	}
}

func TestFormatTextPreservesNestedColorAndStyleStacks(t *testing.T) {
	// Closing </b> must NOT wipe outer <gold>; opening <#rrggbb> must NOT wipe outer <b>.
	comp := FormatText("<gold><b>CHAONIUS</b> Network</gold> <b><red>Alert</red></b>")
	if got := extractPlainText(comp); got != "CHAONIUS Network Alert" {
		t.Fatalf("unexpected plain text: %q", got)
	}

	leaves := collectStyledLeaves(comp)
	if len(leaves) < 3 {
		t.Fatalf("expected at least 3 styled leaves, got %d", len(leaves))
	}
	// Leaf 0: "CHAONIUS" -> Gold + Bold
	if leaves[0].Content != "CHAONIUS" || (leaves[0].S.Color == nil || leaves[0].S.Color.Hex() != color.Gold.Hex()) || leaves[0].S.Bold != c.True {
		t.Fatalf("leaf 0 = %q style=%+v, want Gold + Bold", leaves[0].Content, leaves[0].S)
	}
	// Leaf 1: " Network" -> Gold + NotBold
	if leaves[1].Content != " Network" || (leaves[1].S.Color == nil || leaves[1].S.Color.Hex() != color.Gold.Hex()) || leaves[1].S.Bold != c.NotSet {
		t.Fatalf("leaf 1 = %q style=%+v, want Gold + NotSet Bold", leaves[1].Content, leaves[1].S)
	}
	// Leaf 3: "Alert" -> Red + Bold
	last := leaves[len(leaves)-1]
	if last.Content != "Alert" || (last.S.Color == nil || last.S.Color.Hex() != color.Red.Hex()) || last.S.Bold != c.True {
		t.Fatalf("last leaf = %q style=%+v, want Red + Bold", last.Content, last.S)
	}
}

func TestFormatTextRainbowVariations(t *testing.T) {
	// Standard
	comp1 := FormatText("<rainbow>Running Velocitab Header</rainbow>")
	if text := extractPlainText(comp1); text != "Running Velocitab Header" {
		t.Errorf("Expected 'Running Velocitab Header', got %q", text)
	}

	// Bold wrapping <rainbow> must stay bold on every rune.
	boldRainbow := FormatText("<b><rainbow>Hi!</rainbow></b><gray>.</gray>")
	if text := extractPlainText(boldRainbow); text != "Hi!." {
		t.Fatalf("Expected 'Hi!.', got %q", text)
	}
	leaves := collectStyledLeaves(boldRainbow)
	if len(leaves) < 4 {
		t.Fatalf("expected >=4 leaves, got %d", len(leaves))
	}
	for i := 0; i < 3; i++ {
		if leaves[i].S.Bold != c.True || leaves[i].S.Color == nil {
			t.Fatalf("rune %d %q missing Bold or Color: %+v", i, leaves[i].Content, leaves[i].S)
		}
	}
	if (leaves[3].S.Color == nil || leaves[3].S.Color.Hex() != color.Gray.Hex()) || leaves[3].S.Bold != c.NotSet {
		t.Fatalf("trailing dot style = %+v, want Gray non-bold", leaves[3].S)
	}

	// Inner <b> inside <rainbow> must not leak markup into plain text.
	innerBold := FormatText("<rainbow><b>AB</b>C</rainbow>")
	if text := extractPlainText(innerBold); text != "ABC" {
		t.Fatalf("Expected 'ABC', got %q", text)
	}

	// With phase & reversed & MineDown
	if text := extractPlainText(FormatText("<rainbow:10>Running Velocitab Header</rainbow>")); text != "Running Velocitab Header" {
		t.Errorf("unexpected phase rainbow: %q", text)
	}
	if text := extractPlainText(FormatText("<rainbow:!2>Running Velocitab Header</rainbow>")); text != "Running Velocitab Header" {
		t.Errorf("unexpected reversed rainbow: %q", text)
	}
	if text := extractPlainText(FormatText("&rainbow:20&Running Velocitab Header&r")); text != "Running Velocitab Header" {
		t.Errorf("unexpected MineDown rainbow: %q", text)
	}
}

func TestFormatTextGradientsAndInteractive(t *testing.T) {
	comp1 := FormatText("<gradient:#ff0000:#0000ff>Gradient Test</gradient>")
	if text := extractPlainText(comp1); text != "Gradient Test" {
		t.Errorf("Expected 'Gradient Test', got %q", text)
	}

	comp2 := FormatText("<gradient:#ff0000:#00ff00:#0000ff>Multi Stop</gradient>")
	if text := extractPlainText(comp2); text != "Multi Stop" {
		t.Errorf("Expected 'Multi Stop', got %q", text)
	}

	comp3 := FormatText("<gradient:#999:#fff>Short Hex</gradient>")
	if text := extractPlainText(comp3); text != "Short Hex" {
		t.Errorf("Expected 'Short Hex', got %q", text)
	}

	// Phase shifts first glyph away from stop 0 (#ff0000).
	g0 := collectStyledLeaves(FormatText("<gradient:#ff0000:#0000ff:0>Hello</gradient>"))
	g5 := collectStyledLeaves(FormatText("<gradient:#ff0000:#0000ff:0.5>Hello</gradient>"))
	if g0[0].S.Color.Hex() == g5[0].S.Color.Hex() {
		t.Fatalf("expected gradient phase 0.5 to differ from phase 0: %s vs %s", g0[0].S.Color.Hex(), g5[0].S.Color.Hex())
	}

	// Hover + Click combined with formatted tooltip
	interactive := FormatText("<hover:show_text:'<gold>Visit</gold>'><click:open_url:https://chaonius.192015145.xyz><b>Site</b></click></hover>")
	if got := extractPlainText(interactive); got != "Site" {
		t.Fatalf("unexpected interactive plain text: %q", got)
	}
	leaves := collectStyledLeaves(interactive)
	if len(leaves) == 0 || leaves[0].S.ClickEvent == nil || leaves[0].S.HoverEvent == nil || leaves[0].S.Bold != c.True {
		t.Fatalf("expected ClickEvent, HoverEvent, and Bold on interactive leaf, got %+v", leaves[0].S)
	}
}
