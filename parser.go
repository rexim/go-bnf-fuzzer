package main

import "fmt"

type Expr interface {
	GetLoc() Loc
}

type ExprSymbol struct {
	Loc Loc
	Name string
}

func (expr ExprSymbol) GetLoc() Loc {
	return expr.Loc
}

type ExprString struct {
	Loc Loc
	Text []rune
}

func (expr ExprString) GetLoc() Loc {
	return expr.Loc
}

type ExprAlternation struct {
	Loc Loc
	Variants []Expr
}

func (expr ExprAlternation) GetLoc() Loc {
	return expr.Loc
}

type ExprConcat struct {
	Loc Loc
	Elements []Expr
}

func (expr ExprConcat) GetLoc() Loc {
	return expr.Loc
}

type ExprRepetition struct {
	Loc Loc
	Body Expr
	Lower uint
	Upper uint
}

func (expr ExprRepetition) GetLoc() Loc {
	return expr.Loc
}

type ExprRange struct {
	Loc Loc
	Lower rune
	Upper rune
}

func (expr ExprRange) GetLoc() Loc {
	return expr.Loc
}

func ExpectToken(lexer *Lexer, kind TokenKind) (token Token, err error) {
	token, err = lexer.Next()
	if err != nil {
		return
	}
	if token.Kind != kind {
		err = &DiagErr{
			Loc: token.Loc,
			Err: fmt.Errorf("Expected %s but got %s", TokenKindName[kind], TokenKindName[token.Kind]),
		}
		return
	}
	return
}

const MaxUnspecifiedUpperRepetitionBound = 20

func ParsePrimaryExpr(lexer *Lexer) (expr Expr, err error) {
	var token Token
	token, err = lexer.Next()
	if err != nil {
		return
	}
	switch token.Kind {
	case TokenParenOpen:
		expr, err = ParseExpr(lexer)
		if err != nil {
			return
		}
		_, err = ExpectToken(lexer, TokenParenClose)
		if err != nil {
			return
		}
	case TokenCurlyOpen:
		var body Expr
		body, err = ParseExpr(lexer)
		if err != nil {
			return
		}
		_, err = ExpectToken(lexer, TokenCurlyClose)
		if err != nil {
			return
		}
		expr = ExprRepetition{
			Loc: token.Loc,
			Body: body,
			Lower: 0,
			Upper: MaxUnspecifiedUpperRepetitionBound, // TODO: customizable max unspecified upper repetition bound
		}
	case TokenBracketOpen:
		var body Expr
		body, err = ParseExpr(lexer)
		if err != nil {
			return
		}
		_, err = ExpectToken(lexer, TokenBracketClose)
		if err != nil {
			return
		}
		expr = ExprRepetition{
			Loc: token.Loc,
			Body: body,
			Lower: 0,
			Upper: 1,
		}
	case TokenSymbol:
		expr = ExprSymbol{
			Loc:  token.Loc,
			Name: string(token.Text),
		}
	case TokenValueRange:
		err = &DiagErr{
			Loc: token.Loc,
			Err: fmt.Errorf("TODO: Parsing value ranges is not implemented yet"),
		}
		return
	case TokenString:
		var ellipsis Token
		ellipsis, err = lexer.Peek()
		if err != nil {
			return
		}
		if ellipsis.Kind != TokenEllipsis {
			expr = ExprString{
				Loc: token.Loc,
				Text: token.Text,
			}
			return
		}

		if len(token.Text) != 1 {
			err = &DiagErr{
				Loc: token.Loc,
				Err: fmt.Errorf("The lower boundary of the range is expected to be 1 symbol string. Got %d instead.", len(token.Text)),
			}
			return
		}

		lexer.PeekFull = false
		var upper Token

		upper, err = ExpectToken(lexer, TokenString)
		if err != nil {
			return
		}

		if len(upper.Text) != 1 {
			err = &DiagErr{
				Loc: upper.Loc,
				Err: fmt.Errorf("The upper boundary of the range is expected to be 1 symbol string. Got %d instead.", len(upper.Text)),
			}
			return
		}

		expr = ExprRange{
			Loc: ellipsis.Loc,
			Lower: token.Text[0],
			Upper: upper.Text[0],
		}

	case TokenAsterisk:
		var upper Token
		upper, err = lexer.Peek()
		if err != nil {
			return
		}

		var body Expr
		if upper.Kind != TokenNumber {
			body, err = ParsePrimaryExpr(lexer)
			if err != nil {
				return
			}
			expr = ExprRepetition{
				Loc: token.Loc,
				Lower: 0,
				Upper: MaxUnspecifiedUpperRepetitionBound,
				Body: body,
			}
			return
		}

		body, err = ParsePrimaryExpr(lexer)
		if err != nil {
			return
		}

		expr = ExprRepetition{
			Loc: token.Loc,
			Lower: 0,
			Upper: upper.Number,
			Body: body,
		}
		return

	case TokenNumber:
		var asterisk Token
		asterisk, err = lexer.Peek()
		if err != nil {
			return
		}

		var body Expr

		if asterisk.Kind != TokenAsterisk {
			body, err = ParsePrimaryExpr(lexer)
			if err != nil {
				return
			}
			expr = ExprRepetition{
				Loc: token.Loc,
				Lower: token.Number,
				Upper: token.Number,
				Body: body,
			}
			return
		}

		lexer.PeekFull = false;

		var upper Token
		upper, err = lexer.Peek()

		if upper.Kind != TokenNumber {
			body, err = ParsePrimaryExpr(lexer)
			if err != nil {
				return
			}
			expr = ExprRepetition{
				Loc: asterisk.Loc,
				Lower: token.Number,
				Upper: MaxUnspecifiedUpperRepetitionBound,
				Body: body,
			}
			return
		}

		body, err = ParsePrimaryExpr(lexer)
		if err != nil {
			return
		}
		expr = ExprRepetition{
			Loc: asterisk.Loc,
			Lower: token.Number,
			Upper: upper.Number,
			Body: body,
		}
		return
	default:
		err = &DiagErr{
			Loc: token.Loc,
			Err: fmt.Errorf("Expected start of an expression, but got %s", TokenKindName[token.Kind]),
		}
	}
	return
}

func IsPrimaryStart(kind TokenKind) bool {
	return kind == TokenSymbol ||
		kind == TokenString ||
		kind == TokenBracketOpen ||
		kind == TokenCurlyOpen ||
		kind == TokenParenOpen ||
		kind == TokenNumber ||
		kind == TokenAsterisk ||
		kind == TokenValueRange
}

func ParseConcatExpr(lexer *Lexer) (expr Expr, err error) {
	var primary Expr
	primary, err = ParsePrimaryExpr(lexer)
	if err != nil {
		return
	}

	var token Token
	token, err = lexer.Peek()
	if err != nil {
		return
	}
	if !IsPrimaryStart(token.Kind) {
		expr = primary
		return
	}

	concat := ExprConcat{
		Loc:      primary.GetLoc(),
		Elements: []Expr{primary},
	}

	for err == nil && IsPrimaryStart(token.Kind) {
		var child Expr
		child, err = ParsePrimaryExpr(lexer)
		if err != nil {
			return
		}
		concat.Elements = append(concat.Elements, child)
		token, err = lexer.Peek()
	}

	expr = concat
	return
}

func ParseAltExpr(lexer *Lexer) (expr Expr, err error) {
	var concat Expr
	concat, err = ParseConcatExpr(lexer)
	if err != nil {
		return
	}

	var token Token
	token, err = lexer.Peek()
	if err != nil {
		return
	}
	if token.Kind != TokenAlternation {
		expr = concat
		return
	}

	alt := ExprAlternation{
		Loc:      concat.GetLoc(),
		Variants: []Expr{concat},
	}

	for err == nil && token.Kind == TokenAlternation {
		token, err = ExpectToken(lexer, TokenAlternation)
		if err != nil {
			return
		}
		var child Expr
		child, err = ParseConcatExpr(lexer)
		if err != nil {
			return
		}
		alt.Variants = append(alt.Variants, child)
		token, err = lexer.Peek()
	}

	expr = alt
	return
}

func ParseExpr(lexer *Lexer) (expr Expr, err error) {
	expr, err = ParseAltExpr(lexer)
	return
}
