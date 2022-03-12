package gosql

import "fmt"

type location struct {
	line uint
	col  uint
}

type keyword string

const (
	selectKeyword keyword = "select"
	fromKeyword   keyword = "from"
	asKeyword     keyword = "as"
	tableKeyword  keyword = "table"
	createKeyword keyword = "create"
	insertKeyword keyword = "insert"
	intoKeyword   keyword = "into"
	valuesKeyword keyword = "values"
	intKeyword    keyword = "int"
	textKeyword   keyword = "text"
)

type symbol string

const (
	semicolonSymbol  symbol = ";"
	asteriskSymbol   symbol = "*"
	commaSymbol      symbol = ","
	leftparenSymbol  symbol = "("
	rightparenSymbol symbol = ")"
)

type tokenKind uint

const (
	keywordKind tokenKind = iota
	symbolKind
	identifierKind
	stringKind
	numericKind
)

type token struct {
	value string
	kind  tokenKind
	loc   location
}

type cursor struct {
	pointer uint
	loc     location
}

func (t *token) equals(other *token) bool {
	return t.value == other.value && t.kind == other.kind
}

// A lexer takes a string and a cursor and attempts to
// parse a token. If successful, returns a new token and
// a new cursor.
type lexer func(string, cursor) (*token, cursor, bool)

// Main lexing loop
func lex(source string) ([]*token, error) {
	tokens := []*token{}
	cur := cursor{}
	lexers := []lexer{lexIdentifier, lexKeyword, lexNumeric, lexString, lexSymbol}

lex:
	for cur.pointer < uint(len(source)) {
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
			hint = " after " + tokens[len(tokens)-1].value
		}
		return nil, fmt.Errorf("Unable to lex token %s at %d:%d", hint, cur.loc.line, cur.loc.col)
	}
	return tokens, nil
}

// Lexers for fundamental token types
func lexIdentifier(source string, ic cursor) (*token, cursor, bool)
func lexKeyword(source string, ic cursor) (*token, cursor, bool)

// Attempt to lex a number from the source at the given cursor
func lexNumeric(source string, ic cursor) (*token, cursor, bool) {
	cur := ic
	periodFound := false
	expMarkerFound := false

	for ; cur.pointer < uint(len(source)); cur.pointer++ {
		c := source[cur.pointer]
		cur.loc.col++

		isDigit := c >= '0' && c <= '9'
		isPeriod := c == '.'
		isExpMarker := c == 'e'

		// First glyph must be a digit or a period or this isn't a number and we're done
		if cur.pointer == ic.pointer {
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
			if cur.pointer == uint(len(source)-1) {
				return nil, ic, false
			}

			cNext := source[cur.pointer+1]
			if cNext == '-' || cNext == '+' {
				cur.pointer++
				cur.loc.col++
			}

			continue
		}

		// Not a period, not an expMarker, not a digit? We're done.
		if !isDigit {
			break
		}
	}

	// No characters accumulated
	if cur.pointer == ic.pointer {
		return nil, ic, false
	}

	return &token{
		value: source[ic.pointer:cur.pointer],
		loc:   ic.loc,
		kind:  numericKind,
	}, cur, true
}

// Strings start and end with a single apostrophe, and may contain one apostrophe if followed by another to escape it
func lexString(source string, ic cursor) (*token, cursor, bool) {
	return lexCharacterDelimited(source, ic, '\'')
}

// Lex a sequence of characters delimited by delimiter.
// Handles escaping of delimiter by doubling it (eg 'here''s an escaped apostrophe')
func lexCharacterDelimited(source string, ic cursor, delimiter byte) (*token, cursor, bool) {
	cur := ic

	if len(source[cur.pointer:]) == 0 {
		return nil, ic, false
	}

	if source[cur.pointer] != delimiter {
		return nil, ic, false
	}

	// Found the starting delimiter, advance and look for the next one
	cur.loc.col++
	cur.pointer++

	var value []byte
	for ; cur.pointer < uint(len(source)); cur.pointer++ {
		c := source[cur.pointer]

		if c == delimiter {
			if cur.pointer+1 >= uint(len(source)) || source[cur.pointer+1] != delimiter {
				return &token{
					value: string(value),
					loc:   ic.loc,
					kind:  stringKind,
				}, cur, true
			}
			// The delimiter was escaped, add it as a literal and continue
			value = append(value, delimiter)
			// Skip the second one
			cur.loc.col++
			cur.pointer++
		}

		value = append(value, c)
		cur.loc.col++
	}

	return nil, ic, false
}

func lexSymbol(source string, ic cursor) (*token, cursor, bool)
