package natsconnection

import (
	"strings"
	"unicode"
)

const NatsSubjectNamespace = "scramjet"

func namespace(strValues ...string) string {
	namespaceChunks := []string{NatsSubjectNamespace}
	for _, str := range strValues {
		namespaceChunks = append(namespaceChunks, formatForNamespace(str))
	}
	return strings.Join(namespaceChunks, ".")
}

func formatForNamespace(str string) string {
	formattedStr := []rune{}
	for i, r := range str {
		pi := i - 1
		var pr rune
		if pi >= 0 {
			pr = []rune(str)[pi]
		}
		switch {
		case r >= 'A' && r <= 'Z':
			if pr >= 'a' && pr <= 'z' {
				formattedStr = append(formattedStr, '-', unicode.ToLower(r))
			} else {
				formattedStr = append(formattedStr, r)
			}
		case r >= 'a' && r <= 'z':
			formattedStr = append(formattedStr, r)
		case r >= '0' && r <= '9':
			formattedStr = append(formattedStr, r)
		case r == '-':
			formattedStr = append(formattedStr, '-')
		case r == '_':
			formattedStr = append(formattedStr, '-')
		case r == '.':
			formattedStr = append(formattedStr, '.')
		case r == '*':
			formattedStr = append(formattedStr, '*')
		}
	}
	return string(formattedStr)
}
