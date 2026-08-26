package velocitab

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

func TestFormatTextGradientAndRainbow(t *testing.T) {
	compGradient := FormatText("<gradient:#ff0000:#0000ff>Gradient Test</gradient>")
	textGrad := extractPlainText(compGradient)
	if textGrad != "Gradient Test" {
		t.Errorf("Expected 'Gradient Test', got %q", textGrad)
	}

	compRainbow := FormatText("<rainbow>Rainbow Test</rainbow>")
	textRain := extractPlainText(compRainbow)
	if textRain != "Rainbow Test" {
		t.Errorf("Expected 'Rainbow Test', got %q", textRain)
	}
}
