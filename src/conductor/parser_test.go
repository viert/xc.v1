package conductor

import (
	"testing"
)

func TestParse(t *testing.T) {
	expr := "*rb#master"
	tokens, err := ParseExpression([]rune(expr))
	if err != nil {
		t.Error(err)
	}

	if len(tokens) != 1 {
		t.Error("Number of tokens must be exactly 1")
	}

	token := tokens[0]
	if token.Type != TTypeWorkGroup {
		t.Error("TokenType must be TTypeWorkGroup")
	}

	if token.Value != "rb" {
		t.Errorf("token value expected to be \"rb\", got %s", token.Value)
	}

	if len(token.TagsFilter) != 1 {
		t.Error("token must have exactly 1 tag")
	}

	if token.TagsFilter[0] != "master" {
		t.Errorf("token tag expected to be \"master\", got %s", token.TagsFilter[0])
	}
}

func TestParseWithRegexp(t *testing.T) {
	expr := "%group1/test.*/"
	tokens, err := ParseExpression([]rune(expr))

	if err != nil {
		t.Error(err)
	}

	if len(tokens) != 1 {
		t.Error("Number of tokens must be exactly 1")
	}

	token := tokens[0]
	if token.Type != TTypeGroup {
		t.Error("TokenType must be TTypeWorkGroup")
	}

	if token.Value != "group1" {
		t.Errorf("token value expected to be \"group1\", got %s", token.Value)
	}

	if token.RegexpFilter == nil {
		t.Error("regexp filter should not be nil")
	}

	if token.RegexpFilter.String() != "test.*" {
		t.Errorf("regexp expected to be test.*, but got %s", token.RegexpFilter.String())
	}

}

func TestParseAll(t *testing.T) {
	expr := "*"
	tokens, err := ParseExpression([]rune(expr))

	if err != nil {
		t.Error(err)
	}
	if len(tokens) != 1 {
		t.Error("Number of tokens must be exactly 1")
	}

	token := tokens[0]
	if token.Type != TTypeWorkGroup {
		t.Error("TokenType must be TTypeWorkGroup")
	}

	if token.Value != "" {
		t.Errorf("token value expected to be an empty string, got %s", token.Value)
	}

}
