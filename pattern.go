package scramjet

import (
	"errors"

	"github.com/grafana/regexp"
)

// Pattern is used to compare and match request paths to route patterns.
// Patterns are used by the router to determine which handlers to execute for
// a given request.
type Pattern struct {
	str    string
	regExp *regexp.Regexp
}

// NewPattern creates a new pattern from a string. The string should be a
// valid route pattern. If the string is not a valid route pattern, an error
// is returned.
func NewPattern(patternStr string) (*Pattern, error) {
	patternRegExp, err := regExpFromPattern(patternStr)
	if err != nil {
		return nil, err
	}

	pattern := &Pattern{
		str:    patternStr,
		regExp: patternRegExp,
	}

	return pattern, nil
}

// Match compares a path to the pattern and returns a map of named parameters
// extracted from the path as per the pattern. If the path matches the pattern,
// the second return value will be true. If the path does not match the pattern,
// the second return value will be false.
func (p *Pattern) Match(path string) (MessageParams, bool) {
	matches := p.regExp.FindStringSubmatch(path)
	if len(matches) == 0 {
		return nil, false
	}

	keys := p.regExp.SubexpNames()

	params := make(MessageParams, len(keys))
	for i := 1; i < len(keys); i += 1 {
		if keys[i] != "" {
			params[keys[i]] = matches[i]
		}
	}

	return params, true
}

func (p *Pattern) MatchInto(path string, params *MessageParams) bool {
	matchIndices := p.regExp.FindStringSubmatchIndex(path)
	if len(matchIndices) == 0 {
		return false
	}

	keys := p.regExp.SubexpNames()

	if *params == nil {
		*params = make(map[string]string, len(keys))
	}

	for key := range *params {
		delete(*params, key)
	}
	for i := 1; i < len(keys); i += 1 {
		if keys[i] != "" {
			(*params)[keys[i]] = path[matchIndices[i*2]:matchIndices[i*2+1]]
		}
	}

	return true
}

// String returns the string representation of the pattern.
func (p *Pattern) String() string {
	return p.str
}

type chunkKind int

const (
	unknown chunkKind = iota
	static
	dynamic
	wildcard
)

type chunkModifier int

const (
	single chunkModifier = iota
	optional
	oneOrMore
	zeroOrMore
)

type chunk = struct {
	kind     chunkKind
	modifier chunkModifier
	key      string
	pattern  string
}

// regExpFromPattern converts a route pattern string to a regular expression.
// This is the heart of Pattern and parses the pattern character by character.
func regExpFromPattern(patternStr string) (*regexp.Regexp, error) {
	patternRunes := []rune(patternStr)
	patternRunesLen := len(patternRunes)

	var currentChunk *chunk
	chunks := make([]chunk, 0)
	for i := 0; i < patternRunesLen; i += 1 {
		isLastRune := i+1 == patternRunesLen
		isLastRuneInChunk := isLastRune || patternRunes[i+1] == '/'
		currentRune := patternRunes[i]

		if currentRune == '/' {
			if currentChunk != nil {
				chunks = append(chunks, *currentChunk)
			}
			currentChunk = &chunk{}
			continue
		}
		if currentChunk == nil {
			return nil, errors.New("pattern must start with a leading slash")
		}

		if currentChunk.kind == unknown {
			switch currentRune {
			case ':':
				currentChunk.kind = dynamic
			case '*':
				currentChunk.kind = wildcard
			case '(':
				currentChunk.kind = wildcard
				i -= 1
			default:
				currentChunk.kind = static
				i -= 1
			}
			continue
		}

		if currentRune == '(' {
			if currentChunk.kind == dynamic && currentChunk.key == "" {
				return nil, errors.New("dynamic chunks must have a name")
			}

			if currentChunk.pattern != "" {
				return nil, errors.New("pattern chunks cannot contain multiple subpatterns")
			}

			for j := i + 1; j < patternRunesLen; j += 1 {
				if patternRunes[j] == ')' {
					currentChunk.pattern = string(patternRunes[i+1 : j])
					i = j
					break
				}
			}
			continue
		}

		if isLastRuneInChunk {
			switch currentRune {
			case '?':
				currentChunk.modifier = optional
			case '+':
				currentChunk.modifier = oneOrMore
			case '*':
				currentChunk.modifier = zeroOrMore
			}
			if currentChunk.modifier != single {
				continue
			}
		}

		switch currentChunk.kind {
		case dynamic:
			currentChunk.key += string(currentRune)
		case static:
			currentChunk.pattern += string(currentRune)
		case wildcard:
		}
	}
	if currentChunk != nil {
		chunks = append(chunks, *currentChunk)
	}

	regExpStr := "^"
	for _, currentChunk := range chunks {

		if currentChunk.pattern == "" {
			currentChunk.pattern = "[^\\/]+"
		}

		if currentChunk.kind == static || currentChunk.kind == wildcard {
			switch currentChunk.modifier {
			case single:
				regExpStr += "\\/" + currentChunk.pattern
			case optional:
				regExpStr += "(?:\\/" + currentChunk.pattern + ")?"
			case oneOrMore:
				regExpStr += "\\/" + currentChunk.pattern + "(?:\\/" + currentChunk.pattern + ")*"
			case zeroOrMore:
				regExpStr += "(?:\\/" + currentChunk.pattern + "(?:\\/" + currentChunk.pattern + ")*)?"
			}
		} else if currentChunk.kind == dynamic {
			switch currentChunk.modifier {
			case single:
				regExpStr += "\\/(?P<" + currentChunk.key + ">" + currentChunk.pattern + ")"
			case optional:
				regExpStr += "(?:\\/(?P<" + currentChunk.key + ">" + currentChunk.pattern + "))?"
			case oneOrMore:
				regExpStr += "\\/(?P<" + currentChunk.key + ">(?:" + currentChunk.pattern + ")(?:\\/" + currentChunk.pattern + ")*)"
			case zeroOrMore:
				regExpStr += "(?:\\/(?P<" + currentChunk.key + ">" + currentChunk.pattern + "(?:\\/" + currentChunk.pattern + ")*))?"
			}
		}
	}

	regExpStr += "\\/?$"

	regExp, err := regexp.Compile(regExpStr)
	if err != nil {
		return nil, err
	}

	return regExp, nil
}
