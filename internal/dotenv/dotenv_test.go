package dotenv

import (
	"encoding/json"
	"strings"
	"testing"
)

const sample = "# comment\n\nexport FOO=bar\nBAZ=\"a b\\n$x\"\nQUX='single # not comment'\nPLAIN=hello # trailing\nMULTI=\"line1\nline2\"\n"

func TestParseValues(t *testing.T) {
	f, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"FOO":   "bar",
		"BAZ":   "a b\n$x",
		"QUX":   "single # not comment",
		"PLAIN": "hello",
		"MULTI": "line1\nline2",
	}
	for k, v := range want {
		got, ok := f.Lookup(k)
		if !ok || got != v {
			t.Errorf("%s: got %q,%v want %q", k, got, ok, v)
		}
	}
	if string(f.Bytes()) != sample {
		t.Errorf("round-trip changed content:\n%s", f.Bytes())
	}
}

func TestSetPreservesLayout(t *testing.T) {
	f, _ := Parse([]byte(sample))
	f.Set("FOO", "new value")
	f.Set("NEW", "x")
	want := "# comment\n\nexport FOO='new value'\nBAZ=\"a b\\n$x\"\nQUX='single # not comment'\nPLAIN=hello # trailing\nMULTI=\"line1\nline2\"\nNEW=x\n"
	if got := string(f.Bytes()); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetKeepsInlineComment(t *testing.T) {
	f, _ := Parse([]byte("PORT=5432 # default\nNAME=\"x\"  # quoted\n"))
	f.Set("PORT", "6543")
	f.Set("NAME", "y z")
	want := "PORT=6543 # default\nNAME='y z' # quoted\n"
	if got := string(f.Bytes()); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestUnset(t *testing.T) {
	f, _ := Parse([]byte("A=1\nB=2\nA=3\n"))
	if !f.Unset("A") {
		t.Fatal("expected removal")
	}
	if got := string(f.Bytes()); got != "B=2\n" {
		t.Errorf("got %q", got)
	}
	if f.Unset("ZZZ") {
		t.Error("unexpected removal")
	}
}

func TestQuoteRoundTrip(t *testing.T) {
	for _, v := range []string{"", "plain", "has space", "has 'single'", `has "double"`, "multi\nline", "$dollar", "a#b", "tab\there", " lead"} {
		f, err := Parse([]byte("K=" + Quote(v) + "\n"))
		if err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		if got, _ := f.Lookup("K"); got != v {
			t.Errorf("%q -> %s -> %q", v, Quote(v), got)
		}
	}
}

func TestCRLF(t *testing.T) {
	f, _ := Parse([]byte("A=1\r\nB=2\r\n"))
	f.Set("C", "3")
	if got := string(f.Bytes()); got != "A=1\r\nB=2\r\nC=3\r\n" {
		t.Errorf("got %q", got)
	}
}

func TestExplainAndSetComment(t *testing.T) {
	f, _ := Parse([]byte("# db settings\nDB_HOST=localhost\n\n# first\n#   second\nPORT=1\nNOCOMMENT=x\n"))
	if d, ok := f.Explain("DB_HOST"); !ok || d != "db settings" {
		t.Errorf("DB_HOST: %q %v", d, ok)
	}
	if d, _ := f.Explain("PORT"); d != "first\n  second" {
		t.Errorf("PORT: %q", d)
	}
	if d, ok := f.Explain("NOCOMMENT"); !ok || d != "" {
		t.Errorf("NOCOMMENT: %q %v", d, ok)
	}
	if _, ok := f.Explain("MISSING"); ok {
		t.Error("MISSING should not be found")
	}

	// replace an existing block
	f.SetComment("PORT", "the port")
	// add a block where none existed, after a non-blank line
	f.SetComment("NOCOMMENT", "now\nexplained")
	want := "# db settings\nDB_HOST=localhost\n\n# the port\nPORT=1\n\n# now\n# explained\nNOCOMMENT=x\n"
	if got := string(f.Bytes()); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	// remove a block
	f.SetComment("DB_HOST", "")
	if got := string(f.Bytes()); !strings.HasPrefix(got, "DB_HOST=localhost\n") {
		t.Errorf("remove failed: %q", got)
	}

	all := f.ExplainAll()
	if len(all) != 3 || all[1].Key != "PORT" || all[1].Line != 4 || all[1].Description != "the port" {
		t.Errorf("ExplainAll: %+v", all)
	}
}

func TestMultiLineValues(t *testing.T) {
	src := "A=1\nKEY='line one\nline two\n  indented'\nPEM=\"-----BEGIN-----\nabc\\ndef\n-----END-----\"  # cert\nB=2\n"
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := f.Lookup("KEY"); got != "line one\nline two\n  indented" {
		t.Errorf("single-quoted: %q", got)
	}
	// Inside double quotes a literal newline and an escaped \n both become newlines.
	if got, _ := f.Lookup("PEM"); got != "-----BEGIN-----\nabc\ndef\n-----END-----" {
		t.Errorf("double-quoted: %q", got)
	}
	if got, _ := f.Lookup("B"); got != "2" {
		t.Errorf("entry after multi-line value lost: %q", got)
	}
	if string(f.Bytes()) != src {
		t.Errorf("round-trip changed content:\n%s", f.Bytes())
	}

	// Line numbers account for the extra physical lines.
	all := f.ExplainAll()
	if all[3].Key != "B" || all[3].Line != 8 {
		t.Errorf("ExplainAll line numbers: %+v", all)
	}

	// Editing an unrelated key leaves the multi-line entries untouched.
	f.Set("A", "changed")
	if got := string(f.Bytes()); got != strings.Replace(src, "A=1", "A=changed", 1) {
		t.Errorf("unexpected rewrite:\n%s", got)
	}

	// Rewriting a multi-line value stores it on one line with escapes and
	// still reads back identically.
	f.Set("KEY", "x\ny")
	again, err := Parse(f.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := again.Lookup("KEY"); got != "x\ny" {
		t.Errorf("rewritten multi-line: %q", got)
	}
	if got, _ := again.Lookup("PEM"); got != "-----BEGIN-----\nabc\ndef\n-----END-----" {
		t.Errorf("PEM disturbed: %q", got)
	}
}

func TestUnterminatedQuote(t *testing.T) {
	for _, src := range []string{"A='open\nB=2\n", "A=\"open\nB=2\n"} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func TestSingleQuotedJSON(t *testing.T) {
	oneLine := `{"name": "app", "port": 8080, "tags": ["a", "b"], "nested": {"x": null}}`
	multiLine := "{\n  \"name\": \"app\",\n  \"url\": \"https://example.com/?a=1&b=2#frag\",\n  \"escaped\": \"line\\nbreak\",\n  \"money\": \"$5\"\n}"
	src := "ONE='" + oneLine + "'\nMULTI='" + multiLine + "'\nAFTER=ok\n"

	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	// Single quotes are literal: double quotes, braces, "#", "$", and the
	// two-character sequence \n inside the JSON must all survive untouched.
	if got, _ := f.Lookup("ONE"); got != oneLine {
		t.Errorf("one-line JSON:\n got %q\nwant %q", got, oneLine)
	}
	if got, _ := f.Lookup("MULTI"); got != multiLine {
		t.Errorf("multi-line JSON:\n got %q\nwant %q", got, multiLine)
	}
	if got, _ := f.Lookup("AFTER"); got != "ok" {
		t.Errorf("entry after JSON block: %q", got)
	}
	if string(f.Bytes()) != src {
		t.Errorf("round-trip changed content:\n%s", f.Bytes())
	}

	// Both values must still be valid JSON after parsing.
	for _, k := range []string{"ONE", "MULTI"} {
		v, _ := f.Lookup(k)
		var out map[string]any
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			t.Errorf("%s is not valid JSON after parse: %v", k, err)
		}
	}

	// Writing JSON via Set picks single quotes (the value has double quotes
	// but no single quotes or newlines) and reads back identically.
	f.Set("WRITTEN", oneLine)
	if !strings.Contains(string(f.Bytes()), "WRITTEN='"+oneLine+"'\n") {
		t.Errorf("Set did not single-quote JSON:\n%s", f.Bytes())
	}
	again, err := Parse(f.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := again.Lookup("WRITTEN"); got != oneLine {
		t.Errorf("written JSON round-trip: %q", got)
	}
}
