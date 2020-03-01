package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/google/btree"
)

type Token int

const (
	EndOfLine Token = iota
	KeywordToken
	IntegerToken
	StringToken
	OperatorToken
)

type TokenReader struct {
	R      *bufio.Reader
	AtEOL  bool
	peeked bool
	t      Token
	n      int
	s      string
}

func (tr *TokenReader) ReadRuneEOF() (rune, bool) {
	ch, _, err := tr.R.ReadRune()
	if err == io.EOF {
		return 0, true
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "basic: fatal: %s\n", err)
		os.Exit(1)
	}
	return ch, false
}

func (tr *TokenReader) ReadRune() rune {
	ch, eof := tr.ReadRuneEOF()
	if eof {
		fmt.Fprintf(os.Stderr, "basic: fatal: unexpected EOF")
		os.Exit(1)
	}
	return ch
}

func (tr *TokenReader) UnreadRune() {
	tr.R.UnreadRune()
}

func (tr *TokenReader) ReadToken() (Token, int, string) {
	if tr.peeked {
		tr.peeked = false
		return tr.t, tr.n, tr.s
	}

	tr.AtEOL = false
	for {
		var ch rune
		for {
			ch = tr.ReadRune()
			if ch != ' ' {
				break
			}
		}

		if ch == '\n' {
			tr.AtEOL = true
			return EndOfLine, 0, ""
		} else if ch == '\'' {
			for ch != '\n' {
				ch = tr.ReadRune()
			}
			tr.AtEOL = true
			return EndOfLine, 0, ""
		}

		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			kw := string(ch)
			for {
				ch = tr.ReadRune()
				if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
					kw += string(ch)
				} else if ch == '$' || ch == '%' {
					kw += string(ch)
					break
				} else {
					tr.UnreadRune()
					break
				}
			}
			return KeywordToken, 0, strings.ToUpper(kw)
		}

		if ch >= '0' && ch <= '9' {
			var n int
			for ch >= '0' && ch <= '9' {
				n = n*10 + (int(ch) - '0')
				ch = tr.ReadRune()
			}
			tr.UnreadRune()
			return IntegerToken, n, ""
		}

		if ch == '-' || ch == '+' || ch == '*' || ch == '/' || ch == '(' || ch == ')' ||
			ch == ',' || ch == '=' {
			return OperatorToken, 0, string(ch)
		} else if ch == '<' {
			ch = tr.ReadRune()
			if ch == '=' {
				return OperatorToken, 0, "<="
			} else if ch == '>' {
				return OperatorToken, 0, "<>"
			}
			tr.UnreadRune()
			return OperatorToken, 0, "<"
		} else if ch == '>' {
			if ch == '=' {
				return OperatorToken, 0, ">="
			}
			tr.UnreadRune()
			return OperatorToken, 0, ">"
		}

		if ch == '"' {
			var s string
			for {
				ch = tr.ReadRune()
				if ch == '"' {
					break
				}
				s += string(ch)
			}
			return StringToken, 0, s
		}
	}
}

func (tr *TokenReader) PeekToken() (Token, int, string) {
	if tr.peeked {
		return tr.t, tr.n, tr.s
	}

	tr.t, tr.n, tr.s = tr.ReadToken()
	tr.peeked = true
	return tr.t, tr.n, tr.s
}

type Basic struct {
	Vars map[string]interface{}
	Code *btree.BTree
	W    io.Writer
	ErrW io.Writer
}

func NewBasic(w, errW io.Writer) *Basic {
	b := &Basic{
		W:    w,
		ErrW: errW,
	}
	b.New()
	return b
}

func (b *Basic) Error(tr *TokenReader, msg string) {
	fmt.Fprintln(b.ErrW, msg)
	for !tr.AtEOL {
		tr.ReadToken()
	}
}

func (b *Basic) New() {
	b.Vars = map[string]interface{}{}
	b.Code = btree.New(4)
}

func (b *Basic) Save(fn string) {
	f, err := os.Create(fn)
	if err != nil {
		fmt.Fprintf(b.ErrW, "basic: error: SAVE: %s\n", err)
		return
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	b.Code.Ascend(
		func(item btree.Item) bool {
			line := item.(Line)
			fmt.Fprintf(w, "%d ", line.Number)
			line.Stmt.Print(w)
			fmt.Fprintln(w)
			return true
		})
	w.Flush()
}

func (b *Basic) Load(fn string) bool {
	f, err := os.Open(fn)
	if err != nil {
		fmt.Fprintf(b.ErrW, "basic: error: OPEN: %s\n", err)
		return false
	}
	defer f.Close()

	vars := b.Vars
	code := b.Code
	b.New()

	tr := &TokenReader{
		R: bufio.NewReader(f),
	}

	for {
		for {
			ch, eof := tr.ReadRuneEOF()
			if eof {
				return true
			}
			if ch != ' ' && ch != '\n' {
				break
			}
		}
		tr.UnreadRune()

		t, n, _ := tr.ReadToken()
		if t == IntegerToken {
			stmt, ok := b.CompileStatement(tr, true)
			if ok {
				b.Code.ReplaceOrInsert(Line{n, stmt})
			} else {
				break
			}
		} else {
			b.Error(tr, "basic: error: statement must start with a line number")
			break
		}
	}

	b.Vars = vars
	b.Code = code
	return false
}

type Expr interface {
	fmt.Stringer
	Print(w io.Writer)
	Eval(b *Basic) (interface{}, bool)
}

type ValueExpr struct {
	Value interface{}
}

func (ve ValueExpr) String() string {
	switch v := ve.Value.(type) {
	case int:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	case bool:
		if v {
			return "TRUE"
		} else {
			return "FALSE"
		}
	default:
		panic("unexpected value type")
	}
}

func (ve ValueExpr) Print(w io.Writer) {
	switch v := ve.Value.(type) {
	case int:
		fmt.Fprintf(w, "%d", v)
	case string:
		fmt.Fprintf(w, `"%s"`, v)
	case bool:
		if v {
			fmt.Fprint(w, "TRUE")
		} else {
			fmt.Fprint(w, "FALSE")
		}
	default:
		panic("unexpected value type")
	}
}

func (ve ValueExpr) Eval(b *Basic) (interface{}, bool) {
	return ve.Value, true
}

type VarExpr string

func (ve VarExpr) String() string {
	return string(ve)
}

func (ve VarExpr) Print(w io.Writer) {
	fmt.Fprint(w, string(ve))
}

func (ve VarExpr) Eval(b *Basic) (interface{}, bool) {
	val, ok := b.Vars[string(ve)]
	if !ok {
		fmt.Fprintf(b.ErrW, "basic: error: variable not found: %s\n", ve)
		return nil, false
	}
	return val, true
}

type NegateExpr struct {
	Expr Expr
}

func (ne NegateExpr) String() string {
	return "- " + ne.Expr.String()
}

func (ne NegateExpr) Print(w io.Writer) {
	fmt.Fprintf(w, "- %s", ne.Expr)
}

func (ne NegateExpr) Eval(b *Basic) (interface{}, bool) {
	val, ok := ne.Expr.Eval(b)
	if !ok {
		return nil, false
	}
	n, ok := val.(int)
	if !ok {
		fmt.Fprintln(b.ErrW, "basic: error: expected an integer value")
		return nil, false
	}
	return -n, true
}

type BinaryOp struct {
	StringFunc  func(s1, s2 string) (interface{}, bool)
	IntegerFunc func(n1, n2 int) (interface{}, bool)
}

var BinaryOps = map[string]BinaryOp{
	"+": {
		StringFunc: func(s1, s2 string) (interface{}, bool) {
			return s1 + s2, true
		},
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 + n2, true
		},
	},
	"-": {
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 - n2, true
		},
	},
	"*": {
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 * n2, true
		},
	},
	"/": {
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 / n2, true
		},
	},
	"=": {
		StringFunc: func(s1, s2 string) (interface{}, bool) {
			return s1 == s2, true
		},
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 == n2, true
		},
	},
	"<>": {
		StringFunc: func(s1, s2 string) (interface{}, bool) {
			return s1 != s2, true
		},
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 != n2, true
		},
	},
	"<": {
		StringFunc: func(s1, s2 string) (interface{}, bool) {
			return s1 < s2, true
		},
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 < n2, true
		},
	},
	"<=": {
		StringFunc: func(s1, s2 string) (interface{}, bool) {
			return s1 <= s2, true
		},
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 <= n2, true
		},
	},
	">": {
		StringFunc: func(s1, s2 string) (interface{}, bool) {
			return s1 > s2, true
		},
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 > n2, true
		},
	},
	">=": {
		StringFunc: func(s1, s2 string) (interface{}, bool) {
			return s1 >= s2, true
		},
		IntegerFunc: func(n1, n2 int) (interface{}, bool) {
			return n1 >= n2, true
		},
	},
}

type BinaryExpr struct {
	Name  string
	Op    BinaryOp
	Left  Expr
	Right Expr
}

func (be BinaryExpr) String() string {
	return fmt.Sprintf("%s %s %s", be.Left, be.Name, be.Right)
}

func (be BinaryExpr) Print(w io.Writer) {
	fmt.Fprintf(w, "%s %s %s", be.Left, be.Name, be.Right)
}

func (be BinaryExpr) Eval(b *Basic) (interface{}, bool) {
	val1, ok := be.Left.Eval(b)
	if !ok {
		return nil, false
	}
	val2, ok := be.Right.Eval(b)
	if !ok {
		return nil, false
	}

	switch v1 := val1.(type) {
	case int:
		if be.Op.IntegerFunc == nil {
			fmt.Fprintf(b.ErrW, "basic: error: %s does not work for integers\n", be.Name)
			return nil, false
		}
		n2, ok := val2.(int)
		if !ok {
			fmt.Fprintln(b.ErrW, "basic: error: expected an integer value")
			return nil, false
		}
		return be.Op.IntegerFunc(v1, n2)
	case string:
		if be.Op.StringFunc == nil {
			fmt.Fprintf(b.ErrW, "basic: error: %s does not work for strings\n", be.Name)
			return nil, false
		}
		s2, ok := val2.(string)
		if !ok {
			fmt.Fprintln(b.ErrW, "basic: error: expected a string value")
			return nil, false
		}
		return be.Op.StringFunc(v1, s2)

	case bool:
		fmt.Fprintf(b.ErrW, "basic: error: %s does not work for booleans\n", be.Name)
		return nil, false

	default:
		panic("unexpected value type")
	}
}

func (b *Basic) CompileExpr(tr *TokenReader) (Expr, bool) {
	var e Expr

	t, n, s := tr.ReadToken()
	if t == IntegerToken {
		e = ValueExpr{n}
	} else if t == StringToken {
		e = ValueExpr{s}
	} else if t == KeywordToken {
		e = VarExpr(s)
	} else if t == OperatorToken && s == "-" {
		var ok bool
		e, ok = b.CompileExpr(tr)
		if !ok {
			return nil, false
		}
		e = NegateExpr{e}
	} else if t == OperatorToken && s == "(" {
		var ok bool
		e, ok = b.CompileExpr(tr)
		if !ok {
			return nil, false
		}
		t, _, s = tr.ReadToken()
		if t != OperatorToken || s != ")" {
			b.Error(tr, "basic: error: missing closing ) in expression")
			return nil, false
		}
	} else {
		b.Error(tr, "basic: error: unexpected token in expression")
		return nil, false
	}

	t, _, s = tr.PeekToken()
	if t == OperatorToken {
		if op, ok := BinaryOps[s]; ok {
			tr.ReadToken()
			e2, ok := b.CompileExpr(tr)
			if !ok {
				return nil, false
			}
			e = BinaryExpr{
				Name:  s,
				Op:    op,
				Left:  e,
				Right: e2,
			}
		}
	}

	return e, true
}

const (
	GoSubCtx = iota
	ForCtx
	WhileCtx
)

type Ctx struct {
	Type   int
	Number int
}

type Stmt interface {
	Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx)
	Print(w io.Writer)
}

type EndStmt struct{}

func (_ EndStmt) Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx) {
	return -1, stk
}

func (_ EndStmt) Print(w io.Writer) {
	fmt.Fprint(w, "END")
}

type GoSubStmt int

func (gs GoSubStmt) Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx) {
	return int(gs), append(stk, Ctx{GoSubCtx, ln})
}

func (gs GoSubStmt) Print(w io.Writer) {
	fmt.Fprintf(w, "GOSUB %d", int(gs))
}

type ReturnStmt struct{}

func (_ ReturnStmt) Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx) {
	for len(stk) > 0 {
		ctx := stk[len(stk)-1]
		stk = stk[:len(stk)-1]
		if ctx.Type == GoSubCtx {
			return ctx.Number + 1, stk
		}
	}

	fmt.Fprintln(b.ErrW, "basic: error: RETURN without a GOSUB")
	return -1, stk
}

func (_ ReturnStmt) Print(w io.Writer) {
	fmt.Fprint(w, "RETURN")
}

type GotoStmt int

func (gs GotoStmt) Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx) {
	return int(gs), stk
}

func (gs GotoStmt) Print(w io.Writer) {
	fmt.Fprintf(w, "GOTO %d", int(gs))
}

type RemStmt string

func (_ RemStmt) Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx) {
	return ln + 1, stk
}

func (rs RemStmt) Print(w io.Writer) {
	fmt.Fprint(w, "REM ")
	fmt.Fprint(w, string(rs))
}

type AssignStmt struct {
	Var  string
	Expr Expr
}

func (as AssignStmt) Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx) {
	val, ok := as.Expr.Eval(b)
	if !ok {
		return -1, stk
	}

	if as.Var[len(as.Var)-1] == '$' {
		if _, ok := val.(string); !ok {
			fmt.Fprintln(b.ErrW, "basic: error: expected a string value")
			return -1, stk
		}
	} else if as.Var[len(as.Var)-1] == '%' {
		if _, ok := val.(int); !ok {
			fmt.Fprintln(b.ErrW, "basic: error: expected an integer value")
			return -1, stk
		}
	} else {
		panic("not a string or integer variable")
	}

	b.Vars[as.Var] = val
	return ln + 1, stk
}

func (as AssignStmt) Print(w io.Writer) {
	fmt.Fprintf(w, "%s = ", as.Var)
	as.Expr.Print(w)

}

type PrintStmt struct {
	exprs []Expr
}

func (ps PrintStmt) Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx) {
	for i, e := range ps.exprs {
		if i > 0 {
			fmt.Fprint(b.W, ", ")
		}
		val, ok := e.Eval(b)
		if !ok {
			return -1, stk
		}
		switch v := val.(type) {
		case int:
			fmt.Fprintf(b.W, "%d", v)
		case string:
			fmt.Fprint(b.W, v)
		case bool:
			if v {
				fmt.Fprint(b.W, "TRUE")
			} else {
				fmt.Fprint(b.W, "FALSE")
			}
		default:
			panic("unexpected value type")
		}
	}
	fmt.Fprintln(b.W)
	return ln + 1, stk
}

func (ps PrintStmt) Print(w io.Writer) {
	fmt.Fprint(w, "PRINT ")
	for i, e := range ps.exprs {
		if i > 0 {
			fmt.Fprint(w, ", ")
		}
		e.Print(w)
	}
}

type IfThenStmt struct {
	Test Expr
	Then Stmt
	Else Stmt
}

func (its IfThenStmt) Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx) {
	val, ok := its.Test.Eval(b)
	if !ok {
		return -1, stk
	}
	t, ok := val.(bool)
	if !ok {
		fmt.Fprintln(b.ErrW, "basic: error: expected a boolean value")
		return -1, stk
	}
	if t {
		return its.Then.Execute(b, ln, stk)
	} else if its.Else != nil {
		return its.Else.Execute(b, ln, stk)
	}
	return ln + 1, stk
}

func (its IfThenStmt) Print(w io.Writer) {
	fmt.Fprintf(w, "IF %s THEN ", its.Test)
	its.Then.Print(w)
	if its.Else != nil {
		fmt.Fprint(w, " ELSE ")
		its.Else.Print(w)
	}
}

type IfGotoStmt struct {
	Test   Expr
	Number int
}

func (igs IfGotoStmt) Execute(b *Basic, ln int, stk []Ctx) (int, []Ctx) {
	val, ok := igs.Test.Eval(b)
	if !ok {
		return -1, stk
	}
	t, ok := val.(bool)
	if !ok {
		fmt.Fprintln(b.ErrW, "basic: error: expected a boolean value")
		return -1, stk
	}
	if t {
		return igs.Number, stk
	}
	return ln + 1, stk
}

func (igs IfGotoStmt) Print(w io.Writer) {
	fmt.Fprintf(w, "IF %s GOTO %d", igs.Test, igs.Number)
}

func (b *Basic) CompileKeyword(tr *TokenReader, kw string, full bool) (Stmt, bool) {
	var stmt Stmt

	switch kw {
	case "END":
		stmt = EndStmt{}

	case "FOR":
		// XXX

	case "NEXT":
		// XXX

	case "GOSUB":
		t, n, _ := tr.ReadToken()
		if t != IntegerToken {
			b.Error(tr, "basic: error: missing line number for GOSUB")
			return nil, false
		}
		stmt = GoSubStmt(n)

	case "RETURN":
		stmt = ReturnStmt{}

	case "GOTO":
		t, n, _ := tr.ReadToken()
		if t != IntegerToken {
			b.Error(tr, "basic: error: missing line number for GOTO")
			return nil, false
		}
		stmt = GotoStmt(n)

	case "IF":
		e, ok := b.CompileExpr(tr)
		if !ok {
			return nil, false
		}
		t, _, s := tr.ReadToken()
		if t == KeywordToken && s == "THEN" {
			then, ok := b.CompileStatement(tr, false)
			if !ok {
				return nil, false
			}

			var els Stmt
			t, _, s := tr.PeekToken()
			if t == KeywordToken && s == "ELSE" {
				tr.ReadToken()
				els, ok = b.CompileStatement(tr, false)
				if !ok {
					return nil, false
				}
			}
			stmt = IfThenStmt{
				Test: e,
				Then: then,
				Else: els,
			}
		} else if t == KeywordToken && s == "GOTO" {
			t, n, _ := tr.ReadToken()
			if t != IntegerToken {
				b.Error(tr, "basic: error: missing line number for IF GOTO")
				return nil, false
			}
			stmt = IfGotoStmt{
				Test:   e,
				Number: n,
			}
		} else {
			b.Error(tr, "basic: error: expected IF followed by THEN or GOTO")
			return nil, false
		}

	case "INPUT":
		// XXX

	case "PRINT":
		ps := PrintStmt{}
		for {
			e, ok := b.CompileExpr(tr)
			if !ok {
				return nil, false
			}
			ps.exprs = append(ps.exprs, e)

			t, _, s := tr.PeekToken()
			if t != OperatorToken || s != "," {
				break
			}
			tr.ReadToken()
		}
		stmt = ps

	case "REM":
		var s string
		for {
			ch := tr.ReadRune()
			if ch == '\n' {
				tr.UnreadRune()
				break
			}
			s += string(ch)
		}
		stmt = RemStmt(s)

	case "WHILE":
		// XXX

	case "WEND":
		// XXX

	default:
		if kw[len(kw)-1] == '$' || kw[len(kw)-1] == '%' {
			t, _, op := tr.ReadToken()
			if t != OperatorToken || op != "=" {
				b.Error(tr, "basic: error: expected '=' following variable name")
				return nil, false
			}
			e, ok := b.CompileExpr(tr)
			if !ok {
				return nil, false
			}
			stmt = AssignStmt{kw, e}
		} else {
			b.Error(tr, fmt.Sprintf("basic: error: unknown keyword: %s", kw))
			return nil, false
		}
	}

	if full {
		t, _, _ := tr.ReadToken()
		if t != EndOfLine {
			b.Error(tr, fmt.Sprintf("basic: error: too many argument to keyword: %s", kw))
			return nil, false
		}
	}
	return stmt, true
}

func (b *Basic) CompileStatement(tr *TokenReader, full bool) (Stmt, bool) {
	t, _, kw := tr.ReadToken()
	if t != KeywordToken {
		b.Error(tr, "basic: error: statement must start with a keyword or variable")
		return nil, false
	}
	return b.CompileKeyword(tr, kw, full)
}

type Line struct {
	Number int
	Stmt   Stmt
}

func (l Line) Less(than btree.Item) bool {
	return l.Number < (than.(Line)).Number
}

func (b *Basic) Run() {
	var ln int
	var stk []Ctx

	for {
		var line Line
		b.Code.AscendGreaterOrEqual(Line{ln, nil},
			func(item btree.Item) bool {
				line = item.(Line)
				return false
			})
		if line.Number == 0 {
			break
		}

		ln, stk = line.Stmt.Execute(b, line.Number, stk)
		if ln <= 0 {
			break
		}
	}
}

func readRange(tr *TokenReader, opt bool) (int, int, bool) {
	strt := 0
	end := math.MaxInt32

	t, n, _ := tr.ReadToken()
	if t == IntegerToken {
		strt = n
		t, n, s := tr.ReadToken()
		if t == OperatorToken && s == "-" {
			t, n, _ = tr.ReadToken()
			if t != IntegerToken {
				return 0, 0, false
			}
			end = n

			t, _, _ = tr.ReadToken()
			if t != EndOfLine {
				return 0, 0, false
			}
		} else if t != EndOfLine {
			return 0, 0, false
		}
	} else if t == EndOfLine {
		if !opt {
			return 0, 0, false
		}
	} else {
		return 0, 0, false
	}

	if end < strt {
		return 0, 0, false
	}
	return strt, end, true
}

func (b *Basic) Program(tr *TokenReader) {
	for {
		for {
			ch, eof := tr.ReadRuneEOF()
			if eof {
				return
			}
			if ch != ' ' && ch != '\n' {
				break
			}
		}
		tr.UnreadRune()

		t, n, s := tr.ReadToken()
		if t == IntegerToken {
			stmt, ok := b.CompileStatement(tr, true)
			if ok {
				b.Code.ReplaceOrInsert(Line{n, stmt})
			}
		} else if t == KeywordToken {
			switch s {
			case "DELETE":
				strt, end, ok := readRange(tr, false)
				if !ok {
					b.Error(tr, "basic: error: bad argument to DELETE")
					break
				}

				if end == math.MaxInt32 {
					b.Code.Delete(Line{strt, nil})
				} else {
					var lines []Line

					b.Code.AscendGreaterOrEqual(Line{strt, nil},
						func(item btree.Item) bool {
							line := item.(Line)
							if line.Number > end {
								return false
							}
							lines = append(lines, line)
							return true
						})

					for _, line := range lines {
						b.Code.Delete(line)
					}
				}

			case "EXIT":
				t, _, _ = tr.ReadToken()
				if t != EndOfLine {
					b.Error(tr, "basic: error: EXIT takes no arguments")
				} else {
					return
				}

			case "HELP":
				t, _, _ = tr.ReadToken()
				if t != EndOfLine {
					b.Error(tr, "basic: error: HELP takes no arguments")
					break
				}

				fmt.Fprint(b.W, `
<program> =
    <line-number> <statement>
    ...

<command> =
      <statement>
    | <line-number> <statement>
    | DELETE <line-number> [ '-' <line-number> ] ; delete one or a range of line numbers inclusive
    | EXIT
    | HELP
    | LIST [ <line-number> [ '-' <line-number> ]]
    | LOAD <filename> ; load a program into memory from <filename>
    | NEW ; start over with a new program
    | RUN ; run the program from the beginning
    | SAVE <filename> ; save the program in memory to <filename>

<statement> =
    | END ; end execution of the program
    | <for>
    | GOSUB <line-number> ... RETURN
    | GOTO <line-number>
    | IF <logical-expr> THEN <statement> [ELSE <statement>]
    | IF <logical-expr> GOTO <line-number>
    | INPUT [ <string> ',' ] <variable>
    | <string-variable> '=' <string-expr>
    | <integer-variable> '=' <integer-expr>
    | PRINT <expr> [ ','  ...]
    | REM ... ; comment (remark); ' at the end of the line is also a comment
    | <while>

<for> = ; execute the statements with <variable> going from <start> to <end> inclusively
    FOR <variable> = <start> TO <end> [ STEP <step> ]
    <statement> ...
    NEXT <variable>

<while> =
    WHILE <logical-expr>
    <statement> ...
    WEND

<string> = '"' ... '"'
<integer> = [ '-' ] <digit> ...
<digit> = '0' ... '9'
<variable> = <string-variable> | <integer-variable>
<string-variable> = <name> '$'
<integer-variable> = <name> '%'
<line> = <line-number> <statement>
<integer-expr> =
      <integer-variable>
    | <integer>
    | '-' <expr>
    | <integer-expr> ( '+' | '-' | '*' | '/' ) <integer-expr>
    | <intrinsic> '(' <expr> ... ')'
<logical-expr> =
      <integer-expr> <logical-op> <integer-expr>
    | <string-expr> <logical-op> <string-expr>
<logical-op> = '=' | '<>' | '<' | '>' | '<=' | '>='
<string-expr> =
      <string-variable>
    | <string>
    | <string-expr> '+' <string-expr>
`)

			case "LIST":
				strt, end, ok := readRange(tr, true)
				if !ok {
					b.Error(tr, "basic: error: bad argument to LIST")
					break
				}

				b.Code.AscendGreaterOrEqual(Line{strt, nil},
					func(item btree.Item) bool {
						line := item.(Line)
						if line.Number > end {
							return false
						}
						fmt.Fprintf(b.W, "%d ", line.Number)
						line.Stmt.Print(b.W)
						fmt.Fprintln(b.W)
						return true
					})

			case "LOAD":
				t, _, s = tr.ReadToken()
				if t != StringToken {
					b.Error(tr, "basic: error: SAVE expects one string argument")
					break
				}
				t, _, _ = tr.ReadToken()
				if t != EndOfLine {
					b.Error(tr, "basic: error: SAVE expects one string argument")
					break
				}
				b.Load(s)

			case "NEW":
				t, _, _ = tr.ReadToken()
				if t != EndOfLine {
					b.Error(tr, "basic: error: NEW takes no arguments")
					break
				}
				b.New()

			case "RUN":
				t, _, _ = tr.ReadToken()
				if t != EndOfLine {
					b.Error(tr, "basic: error: RUN takes no arguments")
					break
				}
				b.Run()

			case "SAVE":
				t, _, s = tr.ReadToken()
				if t != StringToken {
					b.Error(tr, "basic: error: SAVE expects one string argument")
					break
				}
				t, _, _ = tr.ReadToken()
				if t != EndOfLine {
					b.Error(tr, "basic: error: SAVE expects one string argument")
					break
				}
				b.Save(s)

			default:
				stmt, ok := b.CompileKeyword(tr, s, true)
				if ok {
					stmt.Execute(b, 0, nil)
				}
			}
		} else {
			b.Error(tr, "basic: error: statement must start with a keyword or variable")
		}
	}
}

func main() {
	if len(os.Args) == 1 {
		fmt.Print(`BASIC
type help for help and exit to exit
`)
		NewBasic(os.Stdout, os.Stderr).Program(
			&TokenReader{
				R: bufio.NewReader(os.Stdin),
			})
	} else {
		b := NewBasic(os.Stdout, os.Stderr)
		if b.Load(os.Args[1]) {
			b.Run()
		}
	}
}
