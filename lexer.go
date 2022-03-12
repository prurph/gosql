package gosql

import (
	"fmt"
	"strings"
)

type Location struct {
	Line uint
	Col  uint
}

type Keyword string

const (
	SelectKeyword Keyword = "select"
	FromKeyword   Keyword = "from"
	AsKeyword     Keyword = "as"
	TableKeyword  Keyword = "table"
	CreateKeyword Keyword = "create"
	InsertKeyword Keyword = "insert"
	IntoKeyword   Keyword = "into"
	ValuesKeyword Keyword = "values"
	IntKeyword    Keyword = "int"
	TextKeyword   Keyword = "text"
)

type Symbol string

const (
	SemicolonSymbol  Symbol = ";"
	AsteriskSymbol   Symbol = "*"
	CommaSymbol      Symbol = ","
	LeftParenSymbol  Symbol = "("
	RightParenSymbol Symbol = ")"
)

type TokenKind uint

const (
	KeywordKind TokenKind = iota
	SymbolKind
	IdentifierKind
	StringKind
	NumericKind
)

type Token struct {
	Value string
	Kind  TokenKind
	Loc   Location
}

type Cursor struct {
	Pointer uint
	Loc     Location
}

func (t *Token) equals(other *Token) bool {
	return t.Value == other.Value && t.Kind == other.Kind
}

// A lexer takes a string and a cursor and attempts to
// parse a token. If successful, returns a new token and
// a new cursor.
type lexer func(string, Cursor) (*Token, Cursor, bool)

// Main lexing loop
func lex(source string) ([]*Token, error) {
	tokens := []*Token{}
	cur := Cursor{}
	lexers := []lexer{lexKeyword, lexSymbol, lexString, lexNumeric, lexIdentifier}

lex:
	for cur.Pointer < uint(len(source)) {
		for _, l := range lexers {
			if token, newCursor, ok := l(source, cur); ok {
				cur = newCursor
				// Omit nil tokens for valid, but empty syntax like newlines
				if token != nil {
					tokens = append(tokens, token)
				}
				continue lex
			}
		}
		hint := ""
		if len(tokens) > 0 {
			hint = " after " + tokens[len(tokens)-1].Value
		}
		return nil, fmt.Errorf("Unable to lex token %s at %d:%d", hint, cur.Loc.Line, cur.Loc.Col)
	}
	return tokens, nil
}

// Attempt to lex an identifier: a double-quoted string, or a group of  characters starting
// with an alphabetical character and possibly containing numbers, underscores, or $. For
// this toy implementation, only ASCII characters are supported.
func lexIdentifier(source string, ic Cursor) (*Token, Cursor, bool) {
	// Double-quoted identifier
	if token, newCursor, ok := lexCharacterDelimited(source, ic, '"'); ok {
		return token, newCursor, true
	}

	cur := ic

	c := source[cur.Pointer]
	// Must start with an alphabetical character
	isAlphabetical := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
	if !isAlphabetical {
		return nil, ic, false
	}
	cur.Pointer++
	cur.Loc.Col++

	value := []byte{c}
	for ; cur.Pointer < uint(len(source)); cur.Pointer++ {
		c = source[cur.Pointer]
		isAlphaNumeric := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c > '0' && c <= '9')
		if isAlphaNumeric || c == '$' || c == '_' {
			value = append(value, c)
			cur.Loc.Col++
			continue
		}
		break
	}

	if len(value) == 0 {
		return nil, ic, false
	}
	return &Token{
		Value: strings.ToLower(string(value)),
		Kind:  IdentifierKind,
		Loc:   ic.Loc,
	}, cur, true
}

func lexKeyword(source string, ic Cursor) (*Token, Cursor, bool) {
	cur := ic
	keywords := []Keyword{
		SelectKeyword,
		FromKeyword,
		AsKeyword,
		TableKeyword,
		CreateKeyword,
		InsertKeyword,
		IntoKeyword,
		ValuesKeyword,
		IntKeyword,
		TextKeyword,
	}

	var options []string
	for _, k := range keywords {
		options = append(options, string(k))
	}

	match := longestMatch(source, ic, options)
	if match == "" {
		return nil, ic, false
	}

	cur.Pointer = ic.Pointer + uint(len(match))
	cur.Loc.Col = ic.Loc.Col + uint(len(match))

	return &Token{
		Value: match,
		Kind:  KeywordKind,
		Loc:   ic.Loc,
	}, cur, true
}

// Attempt to lex a number from the source at the given cursor
func lexNumeric(source string, ic Cursor) (*Token, Cursor, bool) {
	cur := ic
	periodFound := false
	expMarkerFound := false

	for ; cur.Pointer < uint(len(source)); cur.Pointer++ {
		c := source[cur.Pointer]
		cur.Loc.Col++

		isDigit := c >= '0' && c <= '9'
		isPeriod := c == '.'
		isExpMarker := c == 'e'

		// First glyph must be a digit or a period or this isn't a number and we're done
		if cur.Pointer == ic.Pointer {
			if !isDigit && !isPeriod {
				return nil, ic, false
			}
			periodFound = isPeriod
			continue
		}

		// There can only be one period in a number
		if isPeriod {
			if periodFound {
				return nil, ic, false
			}
			periodFound = true
			continue
		}

		// There can only be one expMarker
		if isExpMarker {
			if expMarkerFound {
				return nil, ic, false
			}
			// No periods allowed after expMarker
			periodFound, expMarkerFound = true, true

			// expMarker cannot be the last glyph in the source
			if cur.Pointer == uint(len(source)-1) {
				return nil, ic, false
			}

			cNext := source[cur.Pointer+1]
			if cNext == '-' || cNext == '+' {
				cur.Pointer++
				cur.Loc.Col++
			}

			continue
		}

		// Not a period, not an expMarker, not a digit? We're done.
		if !isDigit {
			break
		}
	}

	// No characters accumulated
	if cur.Pointer == ic.Pointer {
		return nil, ic, false
	}

	return &Token{
		Value: source[ic.Pointer:cur.Pointer],
		Loc:   ic.Loc,
		Kind:  NumericKind,
	}, cur, true
}

// Strings start and end with a single apostrophe, and may contain one apostrophe if followed by another to escape it
func lexString(source string, ic Cursor) (*Token, Cursor, bool) {
	return lexCharacterDelimited(source, ic, '\'')
}

// Lex a sequence of characters delimited by delimiter.
// Handles escaping of delimiter by doubling it (eg 'here''s an escaped apostrophe')
func lexCharacterDelimited(source string, ic Cursor, delimiter byte) (*Token, Cursor, bool) {
	cur := ic

	if len(source[cur.Pointer:]) == 0 {
		return nil, ic, false
	}

	if source[cur.Pointer] != delimiter {
		return nil, ic, false
	}

	// Found the starting delimiter, advance and look for the next one
	cur.Loc.Col++
	cur.Pointer++

	var value []byte
	for ; cur.Pointer < uint(len(source)); cur.Pointer++ {
		c := source[cur.Pointer]

		if c == delimiter {
			if cur.Pointer+1 >= uint(len(source)) || source[cur.Pointer+1] != delimiter {
				return &Token{
					Value: string(value),
					Loc:   ic.Loc,
					Kind:  StringKind,
				}, cur, true
			}
			// The delimiter was escaped, add it as a literal and continue
			value = append(value, delimiter)
			// Skip the second one
			cur.Loc.Col++
			cur.Pointer++
		}

		value = append(value, c)
		cur.Loc.Col++
	}

	return nil, ic, false
}

// Symbols are elements of a fixed set of strings. Also discards whitespace.
func lexSymbol(source string, ic Cursor) (*Token, Cursor, bool) {
	c := source[ic.Pointer]
	cur := ic
	cur.Pointer++
	cur.Loc.Col++

	// Syntax that should be discarded
	switch c {
	case '\n':
		cur.Loc.Line++
		cur.Loc.Col = 0
		fallthrough
	case '\t':
		fallthrough
	case ' ':
		return nil, cur, true
	}

	// Syntax that should be maintained
	symbols := []Symbol{
		CommaSymbol,
		LeftParenSymbol,
		RightParenSymbol,
		SemicolonSymbol,
		AsteriskSymbol,
	}

	// This language would be cooler with .map
	var options []string
	for _, s := range symbols {
		options = append(options, string(s))
	}

	// `cur` has been advanced, so use the original `ic` for this
	match := longestMatch(source, ic, options)
	// Unknown character
	if match == "" {
		return nil, ic, false
	}

	cur.Pointer = ic.Pointer + uint(len(match))
	cur.Loc.Col = ic.Loc.Col + uint(len(match))

	return &Token{
		Value: match,
		Loc:   ic.Loc,
		Kind:  SymbolKind,
	}, cur, true
}

// Iterate through a source string starting at the given cursor to find
// the longest matching substring among the provided options (empty if
// no match).
func longestMatch(source string, ic Cursor, options []string) string {
	var value []byte
	var match string
	skip := map[string]bool{}

	cur := ic

	for cur.Pointer < uint(len(source)) {
		value = append(value, strings.ToLower(string(source[cur.Pointer]))...)
		cur.Pointer++
	match:
		for _, option := range options {
			if skip[option] {
				continue match
			}
			if option == string(value) {
				skip[option] = true
				// The original has a check that this value is longer than
				// any match we've seen so far, but that seems unnecessary
				// given that we're adding characters one at a time so
				// any time we find an option it will always be the longest.
				match = option
				continue
			}

			sharesPrefix := string(value) == option[:cur.Pointer-ic.Pointer]
			tooLong := len(value) > len(option)
			if tooLong || !sharesPrefix {
				skip[option] = true
			}
		}

		if len(skip) == len(options) {
			break
		}
	}

	return match
}
