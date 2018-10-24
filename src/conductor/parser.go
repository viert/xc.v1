package conductor

import (
	"fmt"
	"strings"
)

type TokenType int
type State int

const (
	TTypeHost TokenType = iota
	TTypeGroup
	TTypeWorkGroup
)

const (
	StateWait State = iota
	StateReadHost
	StateReadGroup
	StateReadWorkGroup
	StateReadDatacenter
	StateReadTag
	StateReadCustomFieldKey
	StateReadCustomFieldValue
)

type ConductorToken struct {
	Type              TokenType
	Value             string
	DatacenterFilter  string
	TagsFilter        []string
	CustomFieldFilter []CustomField
	Exclude           bool
}

var (
	hostSymbols = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-"
)

func newToken() *ConductorToken {
	ct := new(ConductorToken)
	ct.TagsFilter = make([]string, 0)
	ct.CustomFieldFilter = make([]CustomField, 0)
	return ct
}

func ParseExpression(expr []rune) ([]*ConductorToken, error) {
	ct := newToken()
	res := make([]*ConductorToken, 0)
	state := StateWait
	tag := ""
	for i := 0; i < len(expr); i++ {
		sym := expr[i]
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

			if sym == ',' {
				if ct.Value == "" {
					return nil, fmt.Errorf("Empty group name at position %d", i)
				}
				res = append(res, ct)
				ct = newToken()
				state = StateWait
				continue
			}

			if sym == '#' {
				state = StateReadTag
				tag = ""
				continue
			}

			ct.Value += string(sym)
		case StateReadWorkGroup:
			if sym == '@' {
				state = StateReadDatacenter
				continue
			}

			if sym == ',' {
				res = append(res, ct)
				ct = newToken()
				state = StateWait
				continue
			}

			if sym == '#' {
				state = StateReadTag
				continue
			}

			ct.Value += string(sym)
		case StateReadHost:
			if sym == ',' {
				res = append(res, ct)
				ct = newToken()
				state = StateWait
				continue
			}

			ct.Value += string(sym)
		case StateReadDatacenter:
			if sym == ',' {
				res = append(res, ct)
				ct = newToken()
				state = StateWait
				continue
			}

			if sym == '#' {
				state = StateReadTag
				continue
			}

			ct.DatacenterFilter += string(sym)

		case StateReadTag:
			if sym == ',' {
				if tag == "" {
					return nil, fmt.Errorf("Empty tag at position %d", i)
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
	if ct.Value != "" {
		res = append(res, ct)
	}

	return res, nil
}
