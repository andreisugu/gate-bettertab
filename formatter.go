package bettertab

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
)

var (
	hexCodeRegex    = regexp.MustCompile(`(?i)&?#([0-9a-f]{6})`)
	tagHexRegex     = regexp.MustCompile(`(?i)<#([0-9a-f]{6})>`)
	tagColorRegex   = regexp.MustCompile(`(?i)<color:#([0-9a-f]{6})>`)
	gradientRegex   = regexp.MustCompile(`(?i)<gradient:#([0-9a-f]{6}):#([0-9a-f]{6})>(.*?)</gradient>`)
	rainbowRegex    = regexp.MustCompile(`(?i)<rainbow>(.*?)</rainbow>`)
	legacyColorMap  = map[rune]color.Color{
		'0': color.Black,
		'1': color.DarkBlue,
		'2': color.DarkGreen,
		'3': color.DarkAqua,
		'4': color.DarkRed,
		'5': color.DarkPurple,
		'6': color.Gold,
		'7': color.Gray,
		'8': color.DarkGray,
		'9': color.Blue,
		'a': color.Green,
		'b': color.Aqua,
		'c': color.Red,
		'd': color.LightPurple,
		'e': color.Yellow,
		'f': color.White,
	}
	namedColorMap = map[string]color.Color{
		"black":        color.Black,
		"dark_blue":    color.DarkBlue,
		"dark_green":   color.DarkGreen,
		"dark_aqua":    color.DarkAqua,
		"dark_red":     color.DarkRed,
		"dark_purple":  color.DarkPurple,
		"gold":         color.Gold,
		"gray":         color.Gray,
		"grey":         color.Gray,
		"dark_gray":    color.DarkGray,
		"dark_grey":    color.DarkGray,
		"blue":         color.Blue,
		"green":        color.Green,
		"aqua":         color.Aqua,
		"red":          color.Red,
		"light_purple": color.LightPurple,
		"pink":         color.LightPurple,
		"yellow":       color.Yellow,
		"white":        color.White,
	}
)

// FormatText parses MiniMessage & Legacy color syntax into a Minecraft Component.
func FormatText(input string) c.Component {
	if input == "" {
		return &c.Text{Content: ""}
	}

	// 1. Process <gradient> tags
	input = gradientRegex.ReplaceAllStringFunc(input, func(m string) string {
		sub := gradientRegex.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		fromHex, toHex, text := sub[1], sub[2], sub[3]
		return applyGradient(text, fromHex, toHex)
	})

	// 2. Process <rainbow> tags
	input = rainbowRegex.ReplaceAllStringFunc(input, func(m string) string {
		sub := rainbowRegex.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		return applyRainbow(sub[1])
	})

	// 3. Convert MiniMessage style & color tags to internal legacy markers
	input = tagHexRegex.ReplaceAllString(input, "&#$1")
	input = tagColorRegex.ReplaceAllString(input, "&#$1")

	for name := range namedColorMap {
		input = strings.ReplaceAll(input, fmt.Sprintf("<%s>", name), namedColorToLegacy(name))
		input = strings.ReplaceAll(input, fmt.Sprintf("</%s>", name), "&r")
	}

	// Style tags
	input = strings.ReplaceAll(input, "<b>", "&l")
	input = strings.ReplaceAll(input, "<bold>", "&l")
	input = strings.ReplaceAll(input, "</b>", "&r")
	input = strings.ReplaceAll(input, "</bold>", "&r")
	input = strings.ReplaceAll(input, "<i>", "&o")
	input = strings.ReplaceAll(input, "<italic>", "&o")
	input = strings.ReplaceAll(input, "</i>", "&r")
	input = strings.ReplaceAll(input, "</italic>", "&r")
	input = strings.ReplaceAll(input, "<u>", "&n")
	input = strings.ReplaceAll(input, "<underlined>", "&n")
	input = strings.ReplaceAll(input, "</u>", "&r")
	input = strings.ReplaceAll(input, "</underlined>", "&r")
	input = strings.ReplaceAll(input, "<s>", "&m")
	input = strings.ReplaceAll(input, "<strikethrough>", "&m")
	input = strings.ReplaceAll(input, "</s>", "&r")
	input = strings.ReplaceAll(input, "</strikethrough>", "&r")
	input = strings.ReplaceAll(input, "<obf>", "&k")
	input = strings.ReplaceAll(input, "<obfuscated>", "&k")
	input = strings.ReplaceAll(input, "</obf>", "&r")
	input = strings.ReplaceAll(input, "</obfuscated>", "&r")
	input = strings.ReplaceAll(input, "<reset>", "&r")
	input = strings.ReplaceAll(input, "</reset>", "&r")
	input = strings.ReplaceAll(input, "<r>", "&r")
	input = strings.ReplaceAll(input, "</r>", "&r")

	// 4. Parse tokenized string into Component tree
	return parseLegacyTokens(input)
}

func namedColorToLegacy(name string) string {
	switch strings.ToLower(name) {
	case "black":
		return "&0"
	case "dark_blue":
		return "&1"
	case "dark_green":
		return "&2"
	case "dark_aqua":
		return "&3"
	case "dark_red":
		return "&4"
	case "dark_purple":
		return "&5"
	case "gold":
		return "&6"
	case "gray", "grey":
		return "&7"
	case "dark_gray", "dark_grey":
		return "&8"
	case "blue":
		return "&9"
	case "green":
		return "&a"
	case "aqua":
		return "&b"
	case "red":
		return "&c"
	case "light_purple", "pink":
		return "&d"
	case "yellow":
		return "&e"
	case "white":
		return "&f"
	default:
		return "&f"
	}
}

func parseLegacyTokens(s string) c.Component {
	root := &c.Text{Content: ""}
	var (
		curColor      color.Color
		bold          c.State
		italic        c.State
		underlined    c.State
		strikethrough c.State
		obfuscated    c.State
		buf           strings.Builder
	)

	flush := func() {
		if buf.Len() > 0 {
			child := &c.Text{
				Content: buf.String(),
				S: c.Style{
					Color:         curColor,
					Bold:          bold,
					Italic:        italic,
					Underlined:    underlined,
					Strikethrough: strikethrough,
					Obfuscated:    obfuscated,
				},
			}
			root.Extra = append(root.Extra, child)
			buf.Reset()
		}
	}

	resetFormat := func() {
		bold = c.NotSet
		italic = c.NotSet
		underlined = c.NotSet
		strikethrough = c.NotSet
		obfuscated = c.NotSet
	}

	i := 0
	runes := []rune(s)
	n := len(runes)

	for i < n {
		r := runes[i]
		if (r == '&' || r == '§') && i+1 < n {
			next := runes[i+1]

			// Check for hex &#RRGGBB
			if next == '#' && i+7 < n {
				hexStr := string(runes[i+2 : i+8])
				if col, err := color.Hex("#" + hexStr); err == nil {
					flush()
					curColor = col
					resetFormat()
					i += 8
					continue
				}
			}

			// Check standard single character formatting
			lower := next
			if lower >= 'A' && lower <= 'Z' {
				lower = lower + ('a' - 'A')
			}

			if col, ok := legacyColorMap[lower]; ok {
				flush()
				curColor = col
				resetFormat()
				i += 2
				continue
			}

			switch lower {
			case 'l':
				flush()
				bold = c.True
				i += 2
				continue
			case 'o':
				flush()
				italic = c.True
				i += 2
				continue
			case 'n':
				flush()
				underlined = c.True
				i += 2
				continue
			case 'm':
				flush()
				strikethrough = c.True
				i += 2
				continue
			case 'k':
				flush()
				obfuscated = c.True
				i += 2
				continue
			case 'r':
				flush()
				curColor = nil
				resetFormat()
				i += 2
				continue
			}
		}

		buf.WriteRune(r)
		i++
	}

	flush()
	return root
}

func applyGradient(text, fromHex, toHex string) string {
	fromR, fromG, fromB := hexToRGB(fromHex)
	toR, toG, toB := hexToRGB(toHex)

	runes := []rune(text)
	length := len(runes)
	if length == 0 {
		return ""
	}
	if length == 1 {
		return fmt.Sprintf("&#%s%s", fromHex, text)
	}

	var b strings.Builder
	for idx, r := range runes {
		ratio := float64(idx) / float64(length-1)
		curR := int(math.Round(float64(fromR) + ratio*float64(toR-fromR)))
		curG := int(math.Round(float64(fromG) + ratio*float64(toG-fromG)))
		curB := int(math.Round(float64(fromB) + ratio*float64(toB-fromB)))
		b.WriteString(fmt.Sprintf("&#%02x%02x%02x%c", curR, curG, curB, r))
	}
	return b.String()
}

func applyRainbow(text string) string {
	runes := []rune(text)
	length := len(runes)
	if length == 0 {
		return ""
	}

	var b strings.Builder
	for idx, r := range runes {
		hue := float64(idx) / float64(length)
		curR, curG, curB := hsvToRGB(hue, 0.85, 1.0)
		b.WriteString(fmt.Sprintf("&#%02x%02x%02x%c", curR, curG, curB, r))
	}
	return b.String()
}

func hexToRGB(hex string) (int, int, int) {
	val, err := strconv.ParseInt(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		return 255, 255, 255
	}
	return int((val >> 16) & 0xFF), int((val >> 8) & 0xFF), int(val & 0xFF)
}

func hsvToRGB(h, s, v float64) (int, int, int) {
	i := math.Floor(h * 6)
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)

	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	case 5:
		r, g, b = v, p, q
	}
	return int(r * 255), int(g * 255), int(b * 255)
}
