package velaros

import (
	"errors"

	"github.com/grafana/regexp"
)

// Pattern represents a compiled route pattern used for matching message paths.
// Patterns support static segments ('/users/list'), named parameters ('/users/:id'),
// wildcards ('/files/**'), and modifiers (:id?, :tags+, :path*). Use NewPattern to
// create patterns from strings.
type Pattern struct {
	str    string
	chunks []chunk
	regExp *regexp.Regexp
}

// NewPattern creates a pattern from a string. Supported syntax: static segments
// ('/users'), named parameters (':id'), wildcards ('*', '**'), and modifiers
// ('?' optional, '+' one or more, '*' zero or more). Examples: '/users/:id',
// '/files/**', '/api/:version?/users'. Returns an error if the pattern string is invalid.
func NewPattern(patternStr string) (*Pattern, error) {
	chunks, err := parsePatternChunks(patternStr)
	if err != nil {
		return nil, err
	}

	patternRegExp, err := regExpFromChunks(chunks)
	if err != nil {
		return nil, err
	}

	pattern := &Pattern{
		str:    patternStr,
		chunks: chunks,
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

// Path creates a path string from the pattern by replacing dynamic segments with
// the provided parameters. If a required parameter is missing, an error is
// returned. Optional segments are only included if their parameters are provided.
// Wildcard segments are replaced with values from the wildcards slice in order.
// If there are more wildcard segments than values in the slice, an error is returned.
func (p *Pattern) Path(params MessageParams, wildcards []string) (string, error) {
	path := ""
	wildcardIndex := 0

	for _, currentChunk := range p.chunks {
		switch currentChunk.kind {
		case static:
			path += "/" + currentChunk.pattern
		case dynamic:
			value, exists := params[currentChunk.key]

			if !exists {
				if currentChunk.modifier == optional || currentChunk.modifier == zeroOrMore {
					continue
				}
				return "", errors.New("missing required parameter: " + currentChunk.key)
			}

			path += "/" + value
		case wildcard:
			if wildcardIndex >= len(wildcards) {
				return "", errors.New("not enough wildcard values provided")
			}
			path += "/" + wildcards[wildcardIndex]
			wildcardIndex++
		}
	}

	if path == "" {
		path = "/"
	}

	return path, nil
}

// MatchInto is like Match but reuses an existing MessageParams map instead of allocating
// a new one. The map is cleared before populating with new parameters. Returns true if the
// path matches the pattern. This is used internally for performance.
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
			startIdx := matchIndices[i*2]
			endIdx := matchIndices[i*2+1]
			if startIdx >= 0 && endIdx >= 0 {
				(*params)[keys[i]] = path[startIdx:endIdx]
			} else {
				(*params)[keys[i]] = ""
			}
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

func parsePatternChunks(patternStr string) ([]chunk, error) {
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

	return chunks, nil
}

// regExpFromChunks converts parsed pattern chunks to a regular expression.
func regExpFromChunks(chunks []chunk) (*regexp.Regexp, error) {
	regExpStr := "^"
	for _, currentChunk := range chunks {

		if currentChunk.pattern == "" {
			currentChunk.pattern = "[^\\/]+"
		}

		switch currentChunk.kind {
		case static, wildcard:
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
		case dynamic:
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
