package bettertab

import (
	"strings"
	"testing"

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

func TestFormatTextLegacyColors(t *testing.T) {
	comp := FormatText("&aGreen &cRed &eYellow &lBold")
	text := extractPlainText(comp)
	if !strings.Contains(text, "Green") || !strings.Contains(text, "Red") {
		t.Errorf("Expected formatted text to contain words, got %q", text)
	}
}

func TestFormatTextMiniMessage(t *testing.T) {
	comp := FormatText("<green>Online</green> <gold>Store</gold> <bold>VIP</bold>")
	text := extractPlainText(comp)
	if !strings.Contains(text, "Online") || !strings.Contains(text, "VIP") {
		t.Errorf("Expected formatted text to contain words, got %q", text)
	}
}

func TestFormatTextRainbowVariations(t *testing.T) {
	// Standard
	comp1 := FormatText("<rainbow>Running Velocitab Header</rainbow>")
	if text := extractPlainText(comp1); text != "Running Velocitab Header" {
		t.Errorf("Expected 'Running Velocitab Header', got %q", text)
	}

	// With phase
	comp2 := FormatText("<rainbow:10>Running Velocitab Header</rainbow>")
	if text := extractPlainText(comp2); text != "Running Velocitab Header" {
		t.Errorf("Expected 'Running Velocitab Header', got %q", text)
	}

	// Reversed with phase
	comp3 := FormatText("<rainbow:!2>Running Velocitab Header</rainbow>")
	if text := extractPlainText(comp3); text != "Running Velocitab Header" {
		t.Errorf("Expected 'Running Velocitab Header', got %q", text)
	}

	// MineDown inline syntax
	comp4 := FormatText("&rainbow:20&Running Velocitab Header&r")
	if text := extractPlainText(comp4); text != "Running Velocitab Header" {
		t.Errorf("Expected 'Running Velocitab Header', got %q", text)
	}
}

func TestFormatTextGradients(t *testing.T) {
	comp1 := FormatText("<gradient:#ff0000:#0000ff>Gradient Test</gradient>")
	if text := extractPlainText(comp1); text != "Gradient Test" {
		t.Errorf("Expected 'Gradient Test', got %q", text)
	}

	// Multi-stop gradient
	comp2 := FormatText("<gradient:#ff0000:#00ff00:#0000ff>Multi Stop</gradient>")
	if text := extractPlainText(comp2); text != "Multi Stop" {
		t.Errorf("Expected 'Multi Stop', got %q", text)
	}

	// Short hex
	comp3 := FormatText("<gradient:#999:#fff>Short Hex</gradient>")
	if text := extractPlainText(comp3); text != "Short Hex" {
		t.Errorf("Expected 'Short Hex', got %q", text)
	}
}
