package input

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

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
	case toonDelimiterComma:
		return ','
	}
	return ','
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
	left, right, ok := splitHeaderColon(content)
	if !ok {
		return toonParsedHeader{}, false, nil
	}
	return parseHeaderBrackets(left, right)
}

func splitHeaderColon(content string) (string, string, bool) {
	colon := indexOutsideQuotes(content, ':')
	if colon == -1 {
		return "", "", false
	}
	left := strings.TrimSpace(content[:colon])
	if left == "" {
		return "", "", false
	}
	return left, strings.TrimSpace(content[colon+1:]), true
}

func parseHeaderBrackets(left, right string) (toonParsedHeader, bool, error) {
	bracketStart := indexOutsideQuotes(left, '[')
	if bracketStart == -1 {
		return toonParsedHeader{}, false, nil
	}
	return extractBracketParts(left, bracketStart, right)
}

func extractBracketParts(left string, bracketStart int, right string) (toonParsedHeader, bool, error) {
	rest := left[bracketStart+1:]
	bracketOffset := indexOutsideQuotes(rest, ']')
	if bracketOffset == -1 {
		return toonParsedHeader{}, false, errors.New("missing closing bracket in array header")
	}
	return assembleHeader(left[:bracketStart], rest[:bracketOffset], rest[bracketOffset+1:], right)
}

func assembleHeader(keyPart, bracketSeg, fieldSeg, right string) (toonParsedHeader, bool, error) {
	header, err := buildHeader(strings.TrimSpace(keyPart), bracketSeg, strings.TrimSpace(fieldSeg))
	if err != nil {
		return toonParsedHeader{}, false, err
	}
	header.inlineValues = right
	return header, true, nil
}

// buildHeader constructs a toonParsedHeader from the key, bracket, and field segments.
func buildHeader(keyPart, bracketSegment, fieldSegment string) (toonParsedHeader, error) {
	header, err := initHeader(keyPart, bracketSegment)
	if err != nil {
		return toonParsedHeader{}, err
	}
	if err := header.setFields(fieldSegment); err != nil {
		return toonParsedHeader{}, err
	}
	return header, nil
}

func initHeader(keyPart, bracketSegment string) (toonParsedHeader, error) {
	var header toonParsedHeader
	header.delimiter = toonDelimiterComma
	if err := header.setKey(keyPart); err != nil {
		return toonParsedHeader{}, err
	}
	if err := header.setBracket(bracketSegment); err != nil {
		return toonParsedHeader{}, err
	}
	return header, nil
}

func (h *toonParsedHeader) setKey(keyPart string) error {
	if keyPart == "" {
		return nil
	}
	key, err := decodeKeyToken(keyPart)
	if err != nil {
		return err
	}
	h.key = key
	return nil
}

func (h *toonParsedHeader) setBracket(segment string) error {
	length, delim, err := parseBracketSegment(segment)
	if err != nil {
		return err
	}
	h.length = length
	h.delimiter = delim
	return nil
}

func (h *toonParsedHeader) setFields(segment string) error {
	if segment == "" {
		return nil
	}
	fields, err := parseFieldSegment(segment, h.delimiter.toRune())
	if err != nil {
		return err
	}
	h.fields = fields
	return nil
}

// parseFieldSegment parses a "{field1,field2}" segment into decoded field names.
func parseFieldSegment(seg string, delim rune) ([]string, error) {
	inner, err := extractFieldInner(seg)
	if err != nil {
		return nil, err
	}
	if inner == "" {
		return nil, nil
	}
	return decodeFieldTokens(inner, delim)
}

func extractFieldInner(seg string) (string, error) {
	if !strings.HasPrefix(seg, "{") || !strings.HasSuffix(seg, "}") {
		return "", errors.New("invalid field segment in array header")
	}
	return seg[1 : len(seg)-1], nil
}

func decodeFieldTokens(inner string, delim rune) ([]string, error) {
	rawFields, err := splitInlineValues(inner, delim)
	if err != nil {
		return nil, err
	}
	return decodeKeyTokens(rawFields)
}

func decodeKeyTokens(rawFields []string) ([]string, error) {
	fields := make([]string, 0, len(rawFields))
	for _, token := range rawFields {
		field, err := decodeKeyToken(token)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func parseBracketSegment(segment string) (int, toonDelimiter, error) {
	segment = strings.TrimPrefix(segment, "#")
	if segment == "" {
		return 0, toonDelimiterComma, errors.New("missing array length")
	}
	digits, delim, err := extractDigitsAndDelim(segment)
	if err != nil {
		return 0, toonDelimiterComma, err
	}
	return parseBracketLength(digits, delim)
}

func extractDigitsAndDelim(segment string) (string, toonDelimiter, error) {
	var sc bracketScanner
	if err := sc.scan(segment); err != nil {
		return "", toonDelimiterComma, err
	}
	return sc.digits.String(), sc.delim, nil
}

type bracketScanner struct {
	digits strings.Builder
	delim  toonDelimiter
}

func (bs *bracketScanner) scan(segment string) error {
	for _, r := range segment {
		if err := bs.scanRune(r); err != nil {
			return err
		}
	}
	return nil
}

func (bs *bracketScanner) scanRune(r rune) error {
	if unicode.IsDigit(r) {
		bs.digits.WriteRune(r)
		return nil
	}
	d, err := parseDelimRune(r)
	if err != nil {
		return err
	}
	bs.delim = d
	return nil
}

func parseDelimRune(r rune) (toonDelimiter, error) {
	switch r {
	case '\t':
		return toonDelimiterTab, nil
	case '|':
		return toonDelimiterPipe, nil
	}
	return toonDelimiterComma, fmt.Errorf("invalid delimiter symbol %q", r)
}

func parseBracketLength(digits string, delim toonDelimiter) (int, toonDelimiter, error) {
	if digits == "" {
		return 0, toonDelimiterComma, errors.New("missing digits in array length")
	}
	length, err := strconv.Atoi(digits)
	if err != nil {
		return 0, toonDelimiterComma, err
	}
	return length, delim, nil
}
