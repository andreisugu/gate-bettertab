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

const endGradientMarker = "</_grad>"

var (
	// Rainbow tag supporting <rainbow>, <rainbow:10>, <rainbow:!2>, <rainbow:!>, <rainbow:reversed:5>
	rainbowTagRegex = regexp.MustCompile(`(?is)<rainbow(?::(!)?(?:reversed)?:?([0-9.-]+)?)?>(.*?)</rainbow(?::[^>]*)?>`)

	// MineDown inline rainbow: &rainbow&Text or &rainbow:20&Text
	mineDownRainbowRegex = regexp.MustCompile(`(?is)&rainbow(?::([0-9.-]+))?&(.*?)(?:&r|&reset|&rainbow&|$)`)

	// Multi-stop gradient regex: <gradient:#c1:#c2:...[:phase]>Text</gradient>
	gradientTagRegex = regexp.MustCompile(`(?is)<gradient:([^>]+)>(.*?)</gradient>`)

	legacyColorMap = map[rune]color.Color{
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

	namedColorHexMap = map[string]string{
		"black":        "#000000",
		"dark_blue":    "#0000aa",
		"dark_green":   "#00aa00",
		"dark_aqua":    "#00aaaa",
		"dark_red":     "#aa0000",
		"dark_purple":  "#aa00aa",
		"gold":         "#ffaa00",
		"gray":         "#aaaaaa",
		"grey":         "#aaaaaa",
		"dark_gray":    "#555555",
		"dark_grey":    "#555555",
		"blue":         "#5555ff",
		"green":        "#55ff55",
		"aqua":         "#55ffff",
		"red":          "#ff5555",
		"light_purple": "#ff55ff",
		"pink":         "#ff55ff",
		"yellow":       "#ffff55",
		"white":        "#ffffff",
	}
)

// FormatText parses MiniMessage, MineDown, Bungee hex (&x&R&R&G&G&B&B), and Legacy color syntax into a native Minecraft Component.
func FormatText(input string) c.Component {
	if input == "" {
		return &c.Text{Content: ""}
	}

	// 1. Handle nested <click:...> and <hover:show_text:...> interactive tags.
	if hasStructuralTag(input) {
		return parseStructuralTags(input)
	}

	// 2. Process Adventure / MiniMessage <rainbow> tags with phase and reverse support.
	input = rainbowTagRegex.ReplaceAllStringFunc(input, func(m string) string {
		sub := rainbowTagRegex.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		isReversed := sub[1] == "!" || strings.Contains(strings.ToLower(m), "reversed")
		phase := 0.0
		if sub[2] != "" {
			if parsed, err := strconv.ParseFloat(sub[2], 64); err == nil {
				phase = parsed
			}
		}
		return applyRainbow(sub[3], phase, isReversed) + endGradientMarker
	})

	// 3. Process MineDown &rainbow[:phase]& inline syntax.
	input = mineDownRainbowRegex.ReplaceAllStringFunc(input, func(m string) string {
		sub := mineDownRainbowRegex.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		phase := 0.0
		if sub[1] != "" {
			if parsed, err := strconv.ParseFloat(sub[1], 64); err == nil {
				phase = parsed
			}
		}
		return applyRainbow(sub[2], phase, false) + endGradientMarker
	})

	// 4. Process <gradient:...> tags (multi-stop and optional phase).
	input = gradientTagRegex.ReplaceAllStringFunc(input, func(m string) string {
		sub := gradientTagRegex.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		return applyMultiGradient(sub[2], sub[1]) + endGradientMarker
	})

	// 5. Parse unified token stream into Component tree preserving nested color/style stacks.
	return parseTokenStream(input)
}

func hasStructuralTag(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "<click:") || strings.Contains(lower, "<hover:")
}

func parseStructuralTags(input string) c.Component {
	root := &c.Text{Content: ""}
	lower := strings.ToLower(input)
	pos := 0
	for pos < len(input) {
		clickIdx := strings.Index(lower[pos:], "<click:")
		hoverIdx := strings.Index(lower[pos:], "<hover:")
		nextIdx := -1
		kind := ""
		switch {
		case clickIdx >= 0 && (hoverIdx < 0 || clickIdx < hoverIdx):
			nextIdx = pos + clickIdx
			kind = "click"
		case hoverIdx >= 0:
			nextIdx = pos + hoverIdx
			kind = "hover"
		}
		if nextIdx < 0 {
			appendSegment(root, FormatText(input[pos:]))
			break
		}
		if nextIdx > pos {
			appendSegment(root, FormatText(input[pos:nextIdx]))
		}
		tagOpen := nextIdx + 1 + len(kind) + 1 // skips "<click:" or "<hover:"
		closeBracket := findUnquotedCloseBracket(input, tagOpen)
		if closeBracket < 0 {
			appendSegment(root, FormatText(input[nextIdx:]))
			break
		}
		header := input[tagOpen:closeBracket]
		bodyStart := closeBracket + 1
		bodyEnd, afterTag := findMatchingClosingTag(lower, kind, bodyStart)
		innerRaw := input[bodyStart:bodyEnd]
		innerComp := FormatText(innerRaw)

		applyStructuralAttribute(innerComp, kind, header)
		appendSegment(root, innerComp)
		pos = afterTag
	}
	if len(root.Extra) == 1 {
		return root.Extra[0]
	}
	return root
}

func appendSegment(root *c.Text, comp c.Component) {
	if comp == nil {
		return
	}
	if t, ok := comp.(*c.Text); ok && t.Content == "" && len(t.Extra) == 0 && t.S.IsZero() {
		return
	}
	root.Extra = append(root.Extra, comp)
}

func findUnquotedCloseBracket(s string, start int) int {
	var quote byte
	for i := start; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == '>' {
			return i
		}
	}
	return -1
}

func findMatchingClosingTag(lower, kind string, start int) (bodyEnd, nextPos int) {
	openTok := "<" + kind + ":"
	closeTok := "</" + kind + ">"
	depth := 1
	i := start
	for i < len(lower) {
		if strings.HasPrefix(lower[i:], closeTok) {
			depth--
			if depth == 0 {
				return i, i + len(closeTok)
			}
			i += len(closeTok)
			continue
		}
		if strings.HasPrefix(lower[i:], openTok) {
			end := findUnquotedCloseBracket(lower, i+len(openTok))
			if end >= 0 {
				depth++
				i = end + 1
				continue
			}
		}
		i++
	}
	return len(lower), len(lower)
}

func applyStructuralAttribute(comp c.Component, kind, header string) {
	actionRaw, valRaw, hasSep := strings.Cut(header, ":")
	if !hasSep {
		return
	}
	action := strings.ToLower(strings.TrimSpace(actionRaw))
	val := unquoteAttr(strings.TrimSpace(valRaw))

	switch kind {
	case "click":
		var clickEv c.ClickEvent
		var defaultHover c.HoverEvent
		switch action {
		case "open_url":
			clickEv = c.OpenUrl(val)
			defaultHover = c.ShowText(FormatText(fmt.Sprintf("&eClick to open link:\n&b%s", val)))
		case "run_command":
			clickEv = c.RunCommand(val)
			defaultHover = c.ShowText(FormatText(fmt.Sprintf("&aClick to run:\n&b%s", val)))
		case "suggest_command":
			clickEv = c.SuggestCommand(val)
			defaultHover = c.ShowText(FormatText(fmt.Sprintf("&eClick to insert:\n&b%s", val)))
		default:
			return
		}
		walkComponents(comp, func(target c.Component) {
			if st := target.Style(); st != nil {
				if st.ClickEvent == nil {
					st.ClickEvent = clickEv
				}
				if st.HoverEvent == nil {
					st.HoverEvent = defaultHover
				}
			}
		})
	case "hover":
		if action != "show_text" {
			return
		}
		hoverEv := c.ShowText(FormatText(val))
		walkComponents(comp, func(target c.Component) {
			if st := target.Style(); st != nil && st.HoverEvent == nil {
				st.HoverEvent = hoverEv
			}
		})
	}
}

func walkComponents(comp c.Component, fn func(c.Component)) {
	if comp == nil {
		return
	}
	fn(comp)
	for _, ch := range comp.Children() {
		walkComponents(ch, fn)
	}
}

func unquoteAttr(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

type styleLayer struct {
	tagDepth int
	inline   bool
}

func (sl styleLayer) state() c.State {
	if sl.tagDepth > 0 || sl.inline {
		return c.True
	}
	return c.NotSet
}

func parseTokenStream(s string) c.Component {
	root := &c.Text{Content: ""}
	var (
		colorStack    []color.Color
		inlineColor   color.Color
		bold          styleLayer
		italic        styleLayer
		underlined    styleLayer
		strikethrough styleLayer
		obfuscated    styleLayer
		buf           strings.Builder
	)

	activeColor := func() color.Color {
		if inlineColor != nil {
			return inlineColor
		}
		if len(colorStack) > 0 {
			return colorStack[len(colorStack)-1]
		}
		return nil
	}

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		child := &c.Text{
			Content: buf.String(),
			S: c.Style{
				Color:         activeColor(),
				Bold:          bold.state(),
				Italic:        italic.state(),
				Underlined:    underlined.state(),
				Strikethrough: strikethrough.state(),
				Obfuscated:    obfuscated.state(),
			},
		}
		root.Extra = append(root.Extra, child)
		buf.Reset()
	}

	resetInlineStyles := func() {
		bold.inline = false
		italic.inline = false
		underlined.inline = false
		strikethrough.inline = false
		obfuscated.inline = false
	}

	resetAllStyles := func() {
		colorStack = nil
		inlineColor = nil
		bold = styleLayer{}
		italic = styleLayer{}
		underlined = styleLayer{}
		strikethrough = styleLayer{}
		obfuscated = styleLayer{}
	}

	runes := []rune(s)
	n := len(runes)
	for i := 0; i < n; {
		r := runes[i]

		// 1. MiniMessage / internal tags <...>
		if r == '<' {
			if closeOffset := findClosingAngle(runes, i); closeOffset > 0 {
				rawTag := string(runes[i+1 : i+closeOffset])
				if handled := applyTagToken(rawTag, flush, &colorStack, &inlineColor,
					&bold, &italic, &underlined, &strikethrough, &obfuscated, resetAllStyles); handled {
					i += closeOffset + 1
					continue
				}
			}
		}

		// 2. Legacy '&' / '§' formatting and hex codes
		if (r == '&' || r == '§') && i+1 < n {
			// Bungee hex: &x&R&R&G&G&B&B
			if (runes[i+1] == 'x' || runes[i+1] == 'X') && i+13 < n {
				if col, ok := parseBungeeHex(runes[i : i+14]); ok {
					flush()
					inlineColor = col
					i += 14
					continue
				}
			}

			// Inline hex: &#RRGGBB
			if runes[i+1] == '#' && i+7 < n {
				hexStr := string(runes[i+2 : i+8])
				if col, err := color.Hex("#" + hexStr); err == nil {
					flush()
					inlineColor = col
					i += 8
					continue
				}
			}

			lower := toLowerASCII(runes[i+1])
			if col, ok := legacyColorMap[lower]; ok {
				flush()
				inlineColor = col
				resetInlineStyles()
				i += 2
				continue
			}

			switch lower {
			case 'l':
				flush()
				bold.inline = true
				i += 2
				continue
			case 'o':
				flush()
				italic.inline = true
				i += 2
				continue
			case 'n':
				flush()
				underlined.inline = true
				i += 2
				continue
			case 'm':
				flush()
				strikethrough.inline = true
				i += 2
				continue
			case 'k':
				flush()
				obfuscated.inline = true
				i += 2
				continue
			case 'r':
				flush()
				inlineColor = nil
				resetInlineStyles()
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

func findClosingAngle(runes []rune, start int) int {
	limit := min(len(runes)-start, 96)
	for off := 1; off < limit; off++ {
		if runes[start+off] == '\n' {
			return -1
		}
		if runes[start+off] == '>' {
			return off
		}
	}
	return -1
}

func applyTagToken(
	rawTag string,
	flush func(),
	colorStack *[]color.Color,
	inlineColor *color.Color,
	bold, italic, underlined, strikethrough, obfuscated *styleLayer,
	resetAll func(),
) bool {
	if rawTag == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(rawTag))

	if lower == "/_grad" {
		flush()
		*inlineColor = nil
		return true
	}

	if after, isClose := strings.CutPrefix(lower, "/"); isClose {
		switch after {
		case "b", "bold":
			flush()
			if bold.tagDepth > 0 {
				bold.tagDepth--
			}
			return true
		case "i", "italic":
			flush()
			if italic.tagDepth > 0 {
				italic.tagDepth--
			}
			return true
		case "u", "underlined":
			flush()
			if underlined.tagDepth > 0 {
				underlined.tagDepth--
			}
			return true
		case "s", "st", "strikethrough":
			flush()
			if strikethrough.tagDepth > 0 {
				strikethrough.tagDepth--
			}
			return true
		case "obf", "obfuscated":
			flush()
			if obfuscated.tagDepth > 0 {
				obfuscated.tagDepth--
			}
			return true
		case "reset", "r":
			flush()
			resetAll()
			return true
		case "color", "colour":
			flush()
			popColor(colorStack, inlineColor)
			return true
		}
		if _, isHex := strings.CutPrefix(after, "#"); isHex {
			flush()
			popColor(colorStack, inlineColor)
			return true
		}
		if _, isNamed := namedColorHexMap[after]; isNamed {
			flush()
			popColor(colorStack, inlineColor)
			return true
		}
		return false
	}

	switch lower {
	case "b", "bold":
		flush()
		bold.tagDepth++
		return true
	case "i", "italic":
		flush()
		italic.tagDepth++
		return true
	case "u", "underlined":
		flush()
		underlined.tagDepth++
		return true
	case "s", "st", "strikethrough":
		flush()
		strikethrough.tagDepth++
		return true
	case "obf", "obfuscated":
		flush()
		obfuscated.tagDepth++
		return true
	case "reset", "r":
		flush()
		resetAll()
		return true
	}

	colorSpec := lower
	if spec, ok := strings.CutPrefix(lower, "color:"); ok {
		colorSpec = spec
	} else if spec, ok := strings.CutPrefix(lower, "colour:"); ok {
		colorSpec = spec
	} else if !strings.HasPrefix(lower, "#") {
		if _, ok := namedColorHexMap[lower]; !ok {
			return false
		}
	}

	if hex, ok := parseColorToHex(colorSpec); ok {
		if col, err := color.Hex(hex); err == nil {
			flush()
			*colorStack = append(*colorStack, col)
			*inlineColor = nil
			return true
		}
	}
	return false
}

func popColor(stack *[]color.Color, inlineColor *color.Color) {
	if len(*stack) > 0 {
		*stack = (*stack)[:len(*stack)-1]
	}
	*inlineColor = nil
}

func parseBungeeHex(seq []rune) (color.Color, bool) {
	if len(seq) != 14 {
		return nil, false
	}
	marker := seq[0]
	if (marker != '&' && marker != '§') || (seq[1] != 'x' && seq[1] != 'X') {
		return nil, false
	}
	var hexBuf [6]byte
	for idx := 0; idx < 6; idx++ {
		sep := seq[2+idx*2]
		digit := seq[3+idx*2]
		if sep != marker && sep != '&' && sep != '§' {
			return nil, false
		}
		if !isHexDigit(digit) {
			return nil, false
		}
		hexBuf[idx] = byte(digit)
	}
	col, err := color.Hex("#" + string(hexBuf[:]))
	if err != nil {
		return nil, false
	}
	return col, true
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func toLowerASCII(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

func applyRainbow(text string, phase float64, reversed bool) string {
	return applyTagAwareColors(text, func(visIdx, visTotal int) string {
		fPhase := phase / 10.0
		center := 128.0
		width := 127.0
		frequency := (math.Pi * 2.0) / float64(max(visTotal, 1))
		if reversed {
			frequency = -frequency
		}
		idx := float64(visIdx)
		curR := clamp(int(math.Round(math.Sin(frequency*idx+2.0+fPhase)*width + center)))
		curG := clamp(int(math.Round(math.Sin(frequency*idx+0.0+fPhase)*width + center)))
		curB := clamp(int(math.Round(math.Sin(frequency*idx+4.0+fPhase)*width + center)))
		return fmt.Sprintf("&#%02x%02x%02x", curR, curG, curB)
	})
}

func applyMultiGradient(text string, colorDefs string) string {
	parts := strings.Split(colorDefs, ":")
	if len(parts) < 2 {
		return text
	}

	var colors [][3]int
	phase := 0.0
	hasPhase := false

	for idx, p := range parts {
		clean := strings.TrimSpace(p)
		if clean == "" {
			continue
		}
		// Only treat a trailing numeric argument as phase after at least two stops.
		if idx == len(parts)-1 && len(colors) >= 2 {
			if parsed, err := strconv.ParseFloat(clean, 64); err == nil {
				phase = parsed
				hasPhase = true
				continue
			}
		}
		if hex, ok := parseColorToHex(clean); ok {
			r, g, b := hexToRGB(hex)
			colors = append(colors, [3]int{r, g, b})
		}
	}

	if len(colors) == 0 {
		return text
	}
	if len(colors) == 1 {
		hex := fmt.Sprintf("&#%02x%02x%02x", colors[0][0], colors[0][1], colors[0][2])
		return applyTagAwareColors(text, func(_, _ int) string { return hex })
	}

	numSegments := len(colors) - 1
	return applyTagAwareColors(text, func(visIdx, visTotal int) string {
		progress := 0.0
		if visTotal > 1 {
			progress = float64(visIdx) / float64(visTotal-1)
		}
		if hasPhase && phase != 0 {
			shifted := math.Mod(math.Abs(progress+phase), 2.0)
			if shifted > 1.0 {
				shifted = 2.0 - shifted
			}
			progress = shifted
		}

		scaledProgress := progress * float64(numSegments)
		segIdx := int(math.Floor(scaledProgress))
		if segIdx >= numSegments {
			segIdx = numSegments - 1
		}
		segProgress := scaledProgress - float64(segIdx)

		c1 := colors[segIdx]
		c2 := colors[segIdx+1]
		curR := clamp(int(math.Round(float64(c1[0]) + segProgress*float64(c2[0]-c1[0]))))
		curG := clamp(int(math.Round(float64(c1[1]) + segProgress*float64(c2[1]-c1[1]))))
		curB := clamp(int(math.Round(float64(c1[2]) + segProgress*float64(c2[2]-c1[2]))))
		return fmt.Sprintf("&#%02x%02x%02x", curR, curG, curB)
	})
}

func applyTagAwareColors(text string, colorAt func(visIdx, visTotal int) string) string {
	type segment struct {
		raw     string
		visible bool
	}

	runes := []rune(text)
	n := len(runes)
	segs := make([]segment, 0, n)
	visTotal := 0

	for i := 0; i < n; {
		r := runes[i]
		if r == '<' {
			if off := findClosingAngle(runes, i); off > 0 && isKnownFormattingTag(string(runes[i+1:i+off])) {
				segs = append(segs, segment{raw: string(runes[i : i+off+1])})
				i += off + 1
				continue
			}
		}
		if (r == '&' || r == '§') && i+1 < n {
			if (runes[i+1] == 'x' || runes[i+1] == 'X') && i+13 < n {
				if _, ok := parseBungeeHex(runes[i : i+14]); ok {
					segs = append(segs, segment{raw: string(runes[i : i+14])})
					i += 14
					continue
				}
			}
			if runes[i+1] == '#' && i+7 < n {
				if _, err := color.Hex("#" + string(runes[i+2:i+8])); err == nil {
					segs = append(segs, segment{raw: string(runes[i : i+8])})
					i += 8
					continue
				}
			}
			lower := toLowerASCII(runes[i+1])
			if _, ok := legacyColorMap[lower]; ok || strings.ContainsRune("klmnor", lower) {
				segs = append(segs, segment{raw: string(runes[i : i+2])})
				i += 2
				continue
			}
		}
		if r == '\n' || r == '\r' {
			segs = append(segs, segment{raw: string(r)})
			i++
			continue
		}
		segs = append(segs, segment{raw: string(r), visible: true})
		visTotal++
		i++
	}

	if visTotal == 0 {
		return text
	}

	var b strings.Builder
	visIdx := 0
	for _, seg := range segs {
		if !seg.visible {
			b.WriteString(seg.raw)
			continue
		}
		b.WriteString(colorAt(visIdx, visTotal))
		b.WriteString(seg.raw)
		visIdx++
	}
	return b.String()
}

func isKnownFormattingTag(rawTag string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawTag))
	trimmed := strings.TrimPrefix(lower, "/")
	switch trimmed {
	case "b", "bold", "i", "italic", "u", "underlined",
		"s", "st", "strikethrough", "obf", "obfuscated",
		"reset", "r", "color", "colour", "_grad":
		return true
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(lower, "color:") || strings.HasPrefix(lower, "colour:") {
		return true
	}
	_, ok := namedColorHexMap[trimmed]
	return ok
}

func parseColorToHex(raw string) (string, bool) {
	clean := strings.TrimSpace(raw)
	if hex, ok := namedColorHexMap[strings.ToLower(clean)]; ok {
		return hex, true
	}
	hasHash := strings.HasPrefix(clean, "#")
	s := strings.TrimPrefix(clean, "#")
	if hasHash && len(s) == 3 {
		s = fmt.Sprintf("%c%c%c%c%c%c", s[0], s[0], s[1], s[1], s[2], s[2])
	}
	if len(s) == 6 {
		if _, err := strconv.ParseInt(s, 16, 32); err == nil {
			return "#" + strings.ToLower(s), true
		}
	}
	return "", false
}

func hexToRGB(hex string) (int, int, int) {
	val, err := strconv.ParseInt(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		return 255, 255, 255
	}
	return int((val >> 16) & 0xFF), int((val >> 8) & 0xFF), int(val & 0xFF)
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
