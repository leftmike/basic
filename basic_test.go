package main

import (
	"bufio"
	"bytes"
	"testing"
)

func TestBasic(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"print 123\n", "123\n"},
		{"print \"def\"\n", "def\n"},

		{"print - 123\n", "-123\n"},
		{"print - \"abc\"\n", "basic: error: expected an integer value\n"},

		{"print 123 + 456\n", "579\n"},
		{"print \"abc\" + \"def\"\n", "abcdef\n"},
		{"print 123 - 456\n", "-333\n"},
		{"print \"abc\" - \"def\"\n", "basic: error: - does not work for strings\n"},
		{"print 123 * 456\n", "56088\n"},
		{"print 1234 / 56\n", "22\n"},

		{"print 12 + 34 * 56\n", "1916\n"},
		{"print (12 + 34) * 56\n", "2576\n"},

		{"print 123 = 456\n", "FALSE\n"},
		{"print 123 = 123\n", "TRUE\n"},
		{"print \"abc\" = \"def\"\n", "FALSE\n"},
		{"print \"abc\" = \"abc\"\n", "TRUE\n"},

		{"abc$ = \"def\"\n", ""},
		{"abc$ = 123\n", "basic: error: expected a string value\n"},
		{"xyz% = 123\n", ""},
		{"xyz% = \"def\"\n", "basic: error: expected an integer value\n"},
		{"abc = 123\n", "basic: error: unknown keyword: ABC\n"},
		{"rem this is a comment\n", ""},
		{"print 123\n", "123\n"},
		{"print \"def\"\n", "def\n"},
		{"abc% = 123\nprint abc%\n", "123\n"},
		{"abc$ = \"def\"\nprint abc$\n", "def\n"},
		{`
abc% = 123
abc$ = "def"
print abc%
print abc$
print abc%, abc$
`, "123\ndef\n123, def\n"},
		{`
10 abc% = 123
20 abc$ = "def"
30 print abc%
40 print abc$
50 print abc%, abc$
run
`, "123\ndef\n123, def\n"},
		{`
10 abc% = 123
20 abc$ = "def"
25 goto 50
30 print abc%
40 print abc$
50 print abc%, abc$
run
`, "123, def\n"},
		{`
10 abc% = 123
20 abc$ = "def"
25 gosub 50
30 print abc%
40 print abc$
45 end
50 print abc%, abc$
55 return
60 print "never ran"
run
`, "123, def\n123\ndef\n"},
		{`
10 abc% = 123
20 abc$ = "def"
30 print abc%
40 print abc$
50 print abc%, abc$
list
`, `10 ABC% = 123
20 ABC$ = "def"
30 PRINT ABC%
40 PRINT ABC$
50 PRINT ABC%, ABC$
`},
		{`
10 abc% = 123
20 abc$ = "def"
30 print abc%
40 print abc$
50 print abc%, abc$
list 40
`, `40 PRINT ABC$
50 PRINT ABC%, ABC$
`},
		{`
10 abc% = 123
20 abc$ = "def"
30 print abc%
40 print abc$
50 print abc%, abc$
list 20 - 40
`, `20 ABC$ = "def"
30 PRINT ABC%
40 PRINT ABC$
`},
		{`
10 abc% = 123
20 abc$ = "def"
25 if abc% = 123 goto 50
30 print abc%
40 print abc$
50 print abc%, abc$
run
`, "123, def\n"},
		{`
10 abc% = 123
20 abc$ = "def"
25 if abc$ < "abc" goto 50
30 print abc%
40 print abc$
50 print abc%, abc$
run
`, "123\ndef\n123, def\n"},
		{`
10 abc% = 123
20 if abc% = 123 then print 234
30 print 456
run
`, "234\n456\n"},
		{`
10 abc% = 123
20 if abc% <> 123 then print 234
30 print 456
run
`, "456\n"},
		{`
10 abc% = 123
20 if abc% <> 123 then print 234 else print 789
30 print 456
run
`, "789\n456\n"},
		{`
10 abc% = 123
20 if abc% = 123 then goto 40 else goto 60
30 print 456
40 print 234
50 end
60 print 345
run
`, "234\n"},
		{`
10 abc% = 123
20 if abc% <> 123 then goto 40 else goto 60
30 print 456
40 print 234
50 end
60 print 345
run
`, "345\n"},
		{`
10 print 234
20 print 345
new
30 print 456
run
`, "456\n"},
		{`
10 abc% = 123
20 abc$ = "def"
30 print abc%
40 print abc$
save "testdata/test.basic"
`, ""},
		{`
load "testdata/test.basic"
run
`, "123\ndef\n"},
		{`
10 abc% = 123
20 abc$ = "def"
30 print abc%
40 print abc$
50 print abc%, abc$
delete 20
list
`, `10 ABC% = 123
30 PRINT ABC%
40 PRINT ABC$
50 PRINT ABC%, ABC$
`},
		{`
10 abc% = 123
20 abc$ = "def"
30 print abc%
40 print abc$
50 print abc%, abc$
delete 20 - 40
list
`, `10 ABC% = 123
50 PRINT ABC%, ABC$
`},
	}

	for _, c := range cases {
		w := &bytes.Buffer{}
		b := NewBasic(w, w)
		tr := &TokenReader{
			R: bufio.NewReader(bytes.NewBufferString(c.in)),
		}
		b.Program(tr)
		out := w.String()
		if out != c.out {
			t.Errorf("program:\n%sgot:\n%swant:\n%s", c.in, out, c.out)
		}
	}
}
