package input

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// quoteState tracks whether a scanner is inside a quoted string.
type quoteState struct {
	inQuotes bool
	escaped  bool
}

func (qs *quoteState) process(r rune) {
	switch {
	case qs.escaped:
		qs.escaped = false
	case r == '\\' && qs.inQuotes:
		qs.escaped = true
	case r == '"':
		qs.inQuotes = !qs.inQuotes
	}
}

// indexOutsideQuotes finds the first occurrence of target not inside quotes.
func indexOutsideQuotes(s string, target rune) int {
	var qs quoteState
	for idx, r := range s {
		if !qs.inQuotes && !qs.escaped && r == target {
			return idx
		}
		qs.process(r)
	}
	return -1
}

// unquoteString removes surrounding quotes and unescapes TOON strings.
func unquoteString(token string) (string, error) {
	if err := validateQuotedString(token); err != nil {
		return "", err
	}
	return unescapeInner(token[1 : len(token)-1])
}

func validateQuotedString(token string) error {
	if len(token) < 2 || token[0] != '"' || token[len(token)-1] != '"' {
		return errors.New("invalid quoted string")
	}
	return nil
}

type unescaper struct {
	b       strings.Builder
	escaped bool
}

func (u *unescaper) processByte(ch byte) error {
	if u.escaped {
		u.escaped = false
		return writeEscape(&u.b, ch)
	}
	if ch == '\\' {
		u.escaped = true
		return nil
	}
	u.b.WriteByte(ch)
	return nil
}

func unescapeInner(inner string) (string, error) {
	u := &unescaper{}
	u.b.Grow(len(inner))
	if err := u.processAll(inner); err != nil {
		return "", err
	}
	return u.result()
}

func (u *unescaper) processAll(s string) error {
	for i := range len(s) {
		if err := u.processByte(s[i]); err != nil {
			return err
		}
	}
	return nil
}

func (u *unescaper) result() (string, error) {
	if u.escaped {
		return "", errors.New("unterminated escape sequence")
	}
	return u.b.String(), nil
}

var escapeMap = map[byte]byte{
	'\\': '\\', '"': '"',
	'n': '\n', 'r': '\r', 't': '\t',
}

// writeEscape writes the unescaped byte for a backslash escape sequence.
func writeEscape(b *strings.Builder, ch byte) error {
	if out, ok := escapeMap[ch]; ok {
		b.WriteByte(out)
		return nil
	}
	return fmt.Errorf("invalid escape sequence \\%c", ch)
}

// inlineSplitter holds state for splitting delimiter-separated values.
type inlineSplitter struct {
	delimiter rune
	tokens    []string
	current   strings.Builder
	qs        quoteState
}

func (sp *inlineSplitter) processRune(r rune) {
	if sp.isQuoteRelated(r) {
		sp.writeAndAdvance(r)
		return
	}
	if r == sp.delimiter && !sp.qs.inQuotes {
		sp.flush()
		return
	}
	sp.current.WriteRune(r)
}

func (sp *inlineSplitter) isQuoteRelated(r rune) bool {
	return sp.qs.escaped || (r == '\\' && sp.qs.inQuotes) || r == '"'
}

func (sp *inlineSplitter) writeAndAdvance(r rune) {
	sp.current.WriteRune(r)
	sp.qs.process(r)
}

func (sp *inlineSplitter) flush() {
	sp.tokens = append(sp.tokens, strings.TrimSpace(sp.current.String()))
	sp.current.Reset()
}

// splitInlineValues tokenizes a delimiter-separated list, respecting quotes.
func splitInlineValues(segment string, delimiter rune) ([]string, error) {
	if strings.TrimSpace(segment) == "" {
		return nil, nil
	}
	return splitWithDelimiter(segment, delimiter)
}

func splitWithDelimiter(segment string, delimiter rune) ([]string, error) {
	sp := inlineSplitter{delimiter: delimiter}
	for _, r := range segment {
		sp.processRune(r)
	}
	if sp.qs.inQuotes {
		return nil, errors.New("unterminated string in delimited values")
	}
	sp.flush()
	return sp.tokens, nil
}

// isValidUnquotedKey reports whether key satisfies the identifier pattern.
func isValidUnquotedKey(key string) bool {
	if key == "" {
		return false
	}
	for pos, r := range key {
		if !isValidKeyRune(r, pos == 0) {
			return false
		}
	}
	return true
}

func isValidKeyRune(r rune, first bool) bool {
	if first {
		return r == '_' || unicode.IsLetter(r)
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.'
}

// looksNumeric reports whether s resembles a numeric literal.
func looksNumeric(s string) bool {
	if s == "" {
		return false
	}
	i := skipLeadingMinus(s)
	if i < 0 {
		return false
	}
	return scanNumber(s, i) == len(s)
}

func skipLeadingMinus(s string) int {
	if s[0] != '-' {
		return 0
	}
	if len(s) == 1 {
		return -1
	}
	return 1
}

func scanNumber(s string, i int) int {
	n := scanDigits(s, i)
	if n == 0 {
		return -1
	}
	return scanExponent(s, scanFraction(s, i+n))
}

// scanDigits returns the number of consecutive ASCII digits starting at pos.
func scanDigits(s string, pos int) int {
	n := 0
	for pos+n < len(s) && s[pos+n] >= '0' && s[pos+n] <= '9' {
		n++
	}
	return n
}

// scanFraction advances past a decimal fraction (.NNN) if present.
// Returns -1 if the fraction is malformed, otherwise the new position.
func scanFraction(s string, i int) int {
	if i >= len(s) || s[i] != '.' {
		return i
	}
	i++ // skip '.'
	n := scanDigits(s, i)
	if n == 0 {
		return len(s) + 1 // malformed: force looksNumeric to return false
	}
	return i + n
}

func isExponentStart(b byte) bool {
	return b == 'e' || b == 'E'
}

func isSign(b byte) bool {
	return b == '+' || b == '-'
}

// scanExponent advances past an exponent (e/E[+/-]NNN) if present.
// Returns len(s)+1 if the exponent is malformed, otherwise the new position.
func scanExponent(s string, i int) int {
	if i >= len(s) || !isExponentStart(s[i]) {
		return i
	}
	return scanExponentDigits(s, i+1)
}

func scanExponentDigits(s string, i int) int {
	if i < len(s) && isSign(s[i]) {
		i++
	}
	n := scanDigits(s, i)
	if n == 0 {
		return len(s) + 1
	}
	return i + n
}

// hasForbiddenLeadingZeros detects 01, 007, -01 etc.
func hasForbiddenLeadingZeros(token string) bool {
	digits := stripLeadingSign(token)
	if len(digits) < 2 || digits[0] != '0' {
		return false
	}
	if strings.Contains(token, ".") || strings.ContainsAny(token, "eE") {
		return false
	}
	return unicode.IsDigit(rune(digits[1]))
}

// stripLeadingSign returns s without a leading '-' prefix.
func stripLeadingSign(s string) string {
	if s != "" && s[0] == '-' {
		return s[1:]
	}
	return s
}

// decodeKeyToken decodes a quoted or unquoted TOON key.
func decodeKeyToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("empty key")
	}
	if token[0] == '"' {
		return unquoteString(token)
	}
	return validateUnquotedKey(token)
}

func validateUnquotedKey(token string) (string, error) {
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
	if v, ok := decodeLiteral(token); ok {
		return v, nil
	}
	return decodeNumericOrString(token)
}

func decodeLiteral(token string) (any, bool) {
	switch token {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null":
		return nil, true
	}
	return nil, false
}

func decodeNumericOrString(token string) (any, error) {
	if hasForbiddenLeadingZeros(token) || !looksNumeric(token) {
		return token, nil
	}
	return parseFloat(token)
}

func parseFloat(token string) (any, error) {
	num, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return nil, err
	}
	if num == 0 {
		num = 0
	}
	return num, nil
}

// splitKeyValue splits "key: value" into decoded key and raw value token.
func splitKeyValue(content string) (string, string, error) {
	colon := indexOutsideQuotes(content, ':')
	if colon == -1 {
		return "", "", errors.New("missing colon after key")
	}
	return decodeKV(content, colon)
}

func decodeKV(content string, colon int) (string, string, error) {
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
	spaces := countLeadingSpaces(line)
	if spaces == len(line) {
		return 0, ""
	}
	return spacesToLevel(spaces, indentSize), line[spaces:]
}

func countLeadingSpaces(line string) int {
	for i := range len(line) {
		if line[i] != ' ' && line[i] != '\t' {
			return i
		}
	}
	return len(line)
}

func spacesToLevel(spaces, indentSize int) int {
	if indentSize <= 0 {
		return spaces
	}
	return spaces / indentSize
}

