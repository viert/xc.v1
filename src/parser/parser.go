package parser

import (
	"fmt"
	"regexp"
	"strings"
)

type TokenType int
type State int

const (
	TTypeHost TokenType = iota
	TTypeGroup
	TTypeWorkGroup
	TTypeHostRegexp
)

const (
	StateWait State = iota
	StateReadHost
	StateReadGroup
	StateReadWorkGroup
	StateReadDatacenter
	StateReadTag
	StateReadHostBracePattern
	StateReadRegexp
)

type Token struct {
	Type             TokenType
	Value            string
	DatacenterFilter string
	TagsFilter       []string
	RegexpFilter     *regexp.Regexp
	Exclude          bool
}

var (
	hostSymbols = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-{}"
)

func newToken() *Token {
	ct := new(Token)
	ct.TagsFilter = make([]string, 0)
	ct.RegexpFilter = nil
	return ct
}

func MaybeAddHost(hostlist *[]string, host string, exclude bool) {
	newHl := *hostlist
	if exclude {
		hIdx := SliceIndex(newHl, host)
		if hIdx >= 0 {
			newHl = append(newHl[:hIdx], newHl[hIdx+1:]...)
		}
	} else {
		newHl = append(newHl, host)
	}
	*hostlist = newHl
}

// ParseExpression syntaxically parses the executer dsl
func ParseExpression(expr []rune) ([]*Token, error) {
	ct := newToken()
	res := make([]*Token, 0)
	state := StateWait
	tag := ""
	re := ""
	last := false
	for i := 0; i < len(expr); i++ {
		sym := expr[i]
		last = i == len(expr)-1
		switch state {
		case StateWait:
			if sym == '-' {
				ct.Exclude = true
				continue
			}

			if sym == '*' {
				state = StateReadWorkGroup
				ct.Type = TTypeWorkGroup
				continue
			}

			if sym == '%' {
				state = StateReadGroup
				ct.Type = TTypeGroup
				continue
			}

			if sym == '/' || sym == '~' {
				state = StateReadHost
				ct.Type = TTypeHostRegexp
				continue
			}

			if strings.ContainsRune(hostSymbols, sym) {
				state = StateReadHost
				ct.Type = TTypeHost
				ct.Value += string(sym)
				continue
			}

			return nil, fmt.Errorf("Invalid symbol %s, expected -, *, %% or a hostname at position %d", string(sym), i)
		case StateReadGroup:

			if sym == '@' {
				state = StateReadDatacenter
				continue
			}

			if sym == '#' {
				state = StateReadTag
				tag = ""
				continue
			}

			if sym == '/' {
				state = StateReadRegexp
				re = ""
				continue
			}

			if sym == ',' || last {
				if last && sym != ',' {
					ct.Value += string(sym)
				}

				if ct.Value == "" {
					return nil, fmt.Errorf("Empty group name at position %d", i)
				}
				res = append(res, ct)
				ct = newToken()
				state = StateWait
				continue
			}

			ct.Value += string(sym)

		case StateReadWorkGroup:

			if sym == '@' {
				state = StateReadDatacenter
				continue
			}

			if sym == '#' {
				tag = ""
				state = StateReadTag
				continue
			}

			if sym == '/' {
				state = StateReadRegexp
				re = ""
				continue
			}

			if sym == ',' || last {
				if last && sym != ',' {
					ct.Value += string(sym)
				}
				res = append(res, ct)
				ct = newToken()
				state = StateWait
				continue
			}

			ct.Value += string(sym)

		case StateReadRegexp:
			if sym == '\\' && !last && expr[i+1] == '/' {
				// screened slash
				re += "/"
				i++
				continue
			}

			if sym == '/' {
				compiled, err := regexp.Compile(re)
				if err != nil {
					return nil, fmt.Errorf("error compiling regexp at %d: %s", i, err)
				}
				ct.RegexpFilter = compiled

				res = append(res, ct)
				ct = newToken()
				state = StateWait
				// regexp should stop with '/EOL' or with '/,'
				// however StateWait doesn't expect a comma, so
				// we skip it:
				if !last && expr[i+1] == ',' {
					i++
				}
				continue
			}
			re += string(sym)

		case StateReadHost:
			if sym == '/' {
				state = StateReadRegexp
				re = ""
				continue
			}

			if sym == '{' {
				state = StateReadHostBracePattern
			}

			if sym == ',' || last {
				if last && sym != ',' {
					ct.Value += string(sym)
				}
				res = append(res, ct)
				ct = newToken()
				state = StateWait
				continue
			}

			ct.Value += string(sym)
		case StateReadHostBracePattern:
			if sym == '{' {
				return nil, fmt.Errorf("nested patterns are not allowed (at %d)", i)
			}
			if sym == '}' {
				state = StateReadHost
			}
			ct.Value += string(sym)

		case StateReadDatacenter:
			if sym == ',' || last {
				if last && sym != ',' {
					ct.DatacenterFilter += string(sym)
				}
				res = append(res, ct)
				ct = newToken()
				state = StateWait
				continue
			}

			if sym == '#' {
				tag = ""
				state = StateReadTag
				continue
			}

			if sym == '/' {
				re = ""
				state = StateReadRegexp
				continue
			}

			ct.DatacenterFilter += string(sym)

		case StateReadTag:

			if sym == ',' || last {
				if last && sym != ',' {
					tag += string(sym)
				}
				if tag == "" {
					return nil, fmt.Errorf("empty tag at position %d", i)
				}

				ct.TagsFilter = append(ct.TagsFilter, tag)
				res = append(res, ct)
				ct = newToken()
				state = StateWait
				continue
			}

			if sym == '#' {
				if tag == "" {
					return nil, fmt.Errorf("Empty tag at position %d", i)
				}
				ct.TagsFilter = append(ct.TagsFilter, tag)
				tag = ""
				continue
			}

			tag += string(sym)
		}

	}

	if ct.Value != "" || state == StateReadWorkGroup {
		// workgroup token can be empty
		res = append(res, ct)
	} else {
		if state != StateWait {
			return nil, fmt.Errorf("unexpected end of expression")
		}
	}

	if state == StateReadDatacenter || state == StateReadTag || state == StateReadHostBracePattern || state == StateReadRegexp {
		return nil, fmt.Errorf("unexpected end of expression")
	}

	return res, nil
}

func SliceIndex(s []string, t string) int {
	for i := 0; i < len(s); i++ {
		if t == s[i] {
			return i
		}
	}
	return -1
}
