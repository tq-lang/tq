package input

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// indexOutsideQuotes finds the first occurrence of target not inside quotes.
func indexOutsideQuotes(s string, target rune) int {
	inQuotes := false
	escaped := false
	for idx, r := range s {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inQuotes:
			escaped = true
		case r == '"':
			inQuotes = !inQuotes
		case !inQuotes && r == target:
			return idx
		}
	}
	return -1
}

// unquoteString removes surrounding quotes and unescapes TOON strings.
func unquoteString(token string) (string, error) {
	if len(token) < 2 || token[0] != '"' || token[len(token)-1] != '"' {
		return "", errors.New("invalid quoted string")
	}
	var b strings.Builder
	b.Grow(len(token) - 2)
	escaped := false
	for i := 1; i < len(token)-1; i++ {
		ch := token[i]
		if escaped {
			switch ch {
			case '\\', '"':
				b.WriteByte(ch)
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				return "", fmt.Errorf("invalid escape sequence \\%c", ch)
			}
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		b.WriteByte(ch)
	}
	if escaped {
		return "", errors.New("unterminated escape sequence")
	}
	return b.String(), nil
}

// splitInlineValues tokenizes a delimiter-separated list, respecting quotes.
func splitInlineValues(segment string, delimiter rune) ([]string, error) {
	if strings.TrimSpace(segment) == "" {
		return nil, nil
	}
	var tokens []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range segment {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && inQuotes:
			current.WriteRune(r)
			escaped = true
		case r == '"':
			current.WriteRune(r)
			inQuotes = !inQuotes
		case r == delimiter && !inQuotes:
			tokens = append(tokens, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if inQuotes {
		return nil, errors.New("unterminated string in delimited values")
	}
	tokens = append(tokens, strings.TrimSpace(current.String()))
	return tokens, nil
}

// isValidUnquotedKey reports whether key satisfies the identifier pattern.
func isValidUnquotedKey(key string) bool {
	if key == "" {
		return false
	}
	for pos, r := range key {
		if pos == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

// looksNumeric reports whether s resembles a numeric literal.
func looksNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	i := 0
	if s[0] == '-' {
		i++
		if i == len(s) {
			return false
		}
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && s[i] == '.' {
		i++
		if i == len(s) || s[i] < '0' || s[i] > '9' {
			return false
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i == len(s) || s[i] < '0' || s[i] > '9' {
			return false
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	return i == len(s)
}

// hasForbiddenLeadingZeros detects 01, 007, -01 etc.
func hasForbiddenLeadingZeros(token string) bool {
	if len(token) < 2 {
		return false
	}
	if token[0] != '0' && (len(token) <= 1 || token[0] != '-' || token[1] != '0') {
		return false
	}
	if strings.Contains(token, ".") || strings.ContainsAny(token, "eE") {
		return false
	}
	if token[0] == '-' {
		return len(token) > 2 && token[1] == '0' && unicode.IsDigit(rune(token[2]))
	}
	return unicode.IsDigit(rune(token[1]))
}

// decodeKeyToken decodes a quoted or unquoted TOON key.
func decodeKeyToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("empty key")
	}
	if token[0] == '"' {
		return unquoteString(token)
	}
	if !isValidUnquotedKey(token) {
		return "", fmt.Errorf("invalid unquoted key %q", token)
	}
	return token, nil
}

// decodePrimitiveToken parses a TOON value token into a Go value.
func decodePrimitiveToken(token string) (any, error) {
	if token == "" {
		return "", nil
	}
	if token[0] == '"' {
		return unquoteString(token)
	}
	switch token {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}
	if hasForbiddenLeadingZeros(token) {
		return token, nil
	}
	if looksNumeric(token) {
		num, err := strconv.ParseFloat(token, 64)
		if err != nil {
			return nil, err
		}
		if num == 0 {
			num = 0
		}
		return num, nil
	}
	return token, nil
}

// splitKeyValue splits "key: value" into decoded key and raw value token.
func splitKeyValue(content string) (string, string, error) {
	colon := indexOutsideQuotes(content, ':')
	if colon == -1 {
		return "", "", errors.New("missing colon after key")
	}
	keyToken := strings.TrimSpace(content[:colon])
	valueToken := strings.TrimSpace(content[colon+1:])
	key, err := decodeKeyToken(keyToken)
	if err != nil {
		return "", "", err
	}
	return key, valueToken, nil
}

// isKeyValue returns true if content contains a colon outside quotes.
func isKeyValue(content string) bool {
	return indexOutsideQuotes(content, ':') > 0
}

// computeIndent returns the indent level and content portion of a line.
// Uses the given indentSize (spaces per level).
func computeIndent(line string, indentSize int) (int, string) {
	spaces := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ':
			spaces++
		case '\t':
			spaces++
		default:
			if indentSize <= 0 {
				return spaces, line[i:]
			}
			return spaces / indentSize, line[i:]
		}
	}
	// All whitespace.
	return 0, ""
}

type toonDelimiter int

const (
	toonDelimiterComma toonDelimiter = iota
	toonDelimiterTab
	toonDelimiterPipe
)

func (d toonDelimiter) toRune() rune {
	switch d {
	case toonDelimiterTab:
		return '\t'
	case toonDelimiterPipe:
		return '|'
	default:
		return ','
	}
}

// toonParsedHeader holds parsed array header info.
type toonParsedHeader struct {
	key          string
	length       int
	delimiter    toonDelimiter
	fields       []string
	inlineValues string
}

// tryParseHeader detects TOON array headers like "key[N]{fields}: values".
func tryParseHeader(content string) (toonParsedHeader, bool, error) {
	colon := indexOutsideQuotes(content, ':')
	if colon == -1 {
		return toonParsedHeader{}, false, nil
	}
	left := strings.TrimSpace(content[:colon])
	right := strings.TrimSpace(content[colon+1:])
	if left == "" {
		return toonParsedHeader{}, false, nil
	}
	bracketStart := indexOutsideQuotes(left, '[')
	if bracketStart == -1 {
		return toonParsedHeader{}, false, nil
	}
	rest := left[bracketStart+1:]
	bracketOffset := indexOutsideQuotes(rest, ']')
	if bracketOffset == -1 {
		return toonParsedHeader{}, false, errors.New("missing closing bracket in array header")
	}
	keyPart := strings.TrimSpace(left[:bracketStart])
	bracketSegment := rest[:bracketOffset]
	fieldSegment := strings.TrimSpace(rest[bracketOffset+1:])

	header := toonParsedHeader{
		delimiter: toonDelimiterComma,
	}

	if keyPart != "" {
		key, err := decodeKeyToken(keyPart)
		if err != nil {
			return toonParsedHeader{}, false, err
		}
		header.key = key
	}

	length, delim, err := parseBracketSegment(bracketSegment)
	if err != nil {
		return toonParsedHeader{}, false, err
	}
	header.length = length
	header.delimiter = delim

	if fieldSegment != "" {
		if !strings.HasPrefix(fieldSegment, "{") || !strings.HasSuffix(fieldSegment, "}") {
			return toonParsedHeader{}, false, errors.New("invalid field segment in array header")
		}
		inner := fieldSegment[1 : len(fieldSegment)-1]
		if inner != "" {
			rawFields, err := splitInlineValues(inner, delim.toRune())
			if err != nil {
				return toonParsedHeader{}, false, err
			}
			fields := make([]string, 0, len(rawFields))
			for _, token := range rawFields {
				field, err := decodeKeyToken(token)
				if err != nil {
					return toonParsedHeader{}, false, err
				}
				fields = append(fields, field)
			}
			header.fields = fields
		}
	}

	header.inlineValues = right
	return header, true, nil
}

func parseBracketSegment(segment string) (int, toonDelimiter, error) {
	segment = strings.TrimPrefix(segment, "#")
	if segment == "" {
		return 0, toonDelimiterComma, errors.New("missing array length")
	}
	var digits strings.Builder
	delim := toonDelimiterComma
	for _, r := range segment {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
			continue
		}
		switch r {
		case '\t':
			delim = toonDelimiterTab
		case '|':
			delim = toonDelimiterPipe
		default:
			return 0, toonDelimiterComma, fmt.Errorf("invalid delimiter symbol %q", r)
		}
	}
	lengthStr := digits.String()
	if lengthStr == "" {
		return 0, toonDelimiterComma, errors.New("missing digits in array length")
	}
	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return 0, toonDelimiterComma, err
	}
	return length, delim, nil
}
