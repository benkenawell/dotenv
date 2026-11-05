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

func TestUnsetRemovesCommentBlock(t *testing.T) {
	cases := []struct{ name, src, key, want string }{
		{"block in middle", "A=1\n\n# about B\n# more\nB=2\n\nC=3\n", "B", "A=1\n\nC=3\n"},
		{"block at end", "A=1\n\n# about B\nB=2\n", "B", "A=1\n"},
		{"block at start", "# about A\nA=1\nB=2\n", "A", "B=2\n"},
		{"no blank separators", "A=1\n# about B\nB=2\nC=3\n", "B", "A=1\nC=3\n"},
		{"header comment separated by blank stays", "# header\n\nA=1\nB=2\n", "A", "# header\n\nB=2\n"},
		{"duplicates each take their block", "# one\nA=1\nB=2\n# two\nA=3\n", "A", "B=2\n"},
		{"inline comment only", "A=1 # inline\nB=2\n", "A", "B=2\n"},
	}
	for _, c := range cases {
		f, err := Parse([]byte(c.src))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !f.Unset(c.key) {
			t.Errorf("%s: key not removed", c.name)
		}
		if got := string(f.Bytes()); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestDisabledEntries(t *testing.T) {
	src := "# header\n\n# about A\nA=1\n\n# about B\n# B='two\n# lines'\n\n# about C\nC=3 # inline\n# NOTE: not=an entry? no, it is\n"
	f, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(f.Bytes()) != src {
		t.Errorf("round-trip changed content:\n%s", f.Bytes())
	}
	if _, ok := f.Lookup("B"); ok {
		t.Error("disabled B must not be an active value")
	}
	if len(f.Entries()) != 2 {
		t.Errorf("Entries should skip disabled: %+v", f.Entries())
	}
	dis := f.DisabledEntries()
	if len(dis) != 1 || dis[0].Key != "B" || dis[0].Value != "two\nlines" {
		t.Errorf("DisabledEntries: %+v", dis)
	}
	// Explain blocks: the disabled entry owns its block and ends it, so C's
	// block is not polluted by B's commented-out line.
	if d, ok := f.Explain("B"); !ok || d != "about B" {
		t.Errorf("explain disabled B: %q %v", d, ok)
	}
	if d, _ := f.Explain("C"); d != "about C" {
		t.Errorf("explain C: %q", d)
	}
	all := f.ExplainAll()
	if len(all) != 3 || !all[1].Disabled || all[1].Key != "B" || all[1].Line != 7 {
		t.Errorf("ExplainAll: %+v", all)
	}
	// "# NOTE: ..." has an invalid key and stays a plain comment.
	if _, ok := f.Explain("NOTE:"); ok {
		t.Error("invalid key must not become a disabled entry")
	}

	// Enable B in place: block preserved, multi-line value restored.
	if !f.Enable("B") {
		t.Fatal("enable B")
	}
	wantEnabled := strings.Replace(src, "# B='two\n# lines'", "B='two\nlines'", 1)
	if got := string(f.Bytes()); got != wantEnabled {
		t.Errorf("enable:\n got %q\nwant %q", got, wantEnabled)
	}
	if v, _ := f.Lookup("B"); v != "two\nlines" {
		t.Errorf("enabled value: %q", v)
	}
	// Disable it again: identical to the original.
	if !f.Disable("B") {
		t.Fatal("disable B")
	}
	if got := string(f.Bytes()); got != src {
		t.Errorf("disable did not round-trip:\n got %q\nwant %q", got, src)
	}

	// Disable an entry with export prefix and inline comment, then enable.
	g, _ := Parse([]byte("export K=v # note\n"))
	g.Disable("K")
	if got := string(g.Bytes()); got != "# export K=v # note\n" {
		t.Errorf("disable export: %q", got)
	}
	g.Enable("K")
	if got := string(g.Bytes()); got != "export K=v # note\n" {
		t.Errorf("enable export: %q", got)
	}

	// Idempotence and missing keys.
	if !f.Disable("B") || !f.Enable("A") {
		t.Error("toggling into the current state should still report found")
	}
	if f.Disable("ZZZ") || f.Enable("ZZZ") {
		t.Error("missing key must report not found")
	}

	// Set on a disabled key re-enables it in place with the new value.
	f.Set("B", "new")
	if got := string(f.Bytes()); !strings.Contains(got, "# about B\nB=new\n") || strings.Contains(got, "# B=") {
		t.Errorf("set on disabled:\n%s", got)
	}

	// Remove takes disabled copies and their blocks.
	f.Disable("B")
	if !f.Unset("B") {
		t.Fatal("unset disabled B")
	}
	if got := string(f.Bytes()); strings.Contains(got, "B") {
		t.Errorf("remove left disabled B behind:\n%s", got)
	}
}

func TestToggle(t *testing.T) {
	f, _ := Parse([]byte("A=1\n# B=2\n"))
	if !f.Toggle("A") || !f.Toggle("B") {
		t.Fatal("toggle existing keys")
	}
	if got := string(f.Bytes()); got != "# A=1\nB=2\n" {
		t.Errorf("after toggle: %q", got)
	}
	if !f.Toggle("A") || !f.Toggle("B") {
		t.Fatal("toggle back")
	}
	if got := string(f.Bytes()); got != "A=1\n# B=2\n" {
		t.Errorf("after second toggle: %q", got)
	}
	if f.Toggle("ZZZ") {
		t.Error("missing key must report not found")
	}
	// With both an active and a disabled copy, toggle disables (the key is
	// active), so the result is unambiguous.
	g, _ := Parse([]byte("# K=old\nK=new\n"))
	g.Toggle("K")
	if got := string(g.Bytes()); got != "# K=old\n# K=new\n" {
		t.Errorf("mixed toggle: %q", got)
	}
}

func TestFormat(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			"disabled duplicate moves up with its block",
			"# db\nDB=1\n\nOTHER=2\n\n# db staging\n# DB=3\n",
			"# db\nDB=1\n# db staging\n# DB=3\n\nOTHER=2\n",
		},
		{
			"order of occurrences preserved, blocks stay with each",
			"# first\n# A=1\nB=x\n# second\nA=2\nC=y\n# third\n# A=3\n",
			"# first\n# A=1\n# second\nA=2\n# third\n# A=3\nB=x\nC=y\n",
		},
		{
			"first occurrence without block, later with",
			"A=1\nB=2\n\n# later\nA=3\n",
			"A=1\n# later\nA=3\nB=2\n",
		},
		{
			"header comment separated by blank is filler and stays",
			"# header\n\nA=1\nB=2\nA=3\n",
			"# header\n\nA=1\nA=3\nB=2\n",
		},
		{
			"multiple keys interleaved",
			"A=1\nB=1\nA=2\nB=2\n",
			"A=1\nA=2\nB=1\nB=2\n",
		},
		{
			"doubled blank collapsed in the middle",
			"A=1\n\nB=2\n\nA=3\n\nC=4\n",
			"A=1\nA=3\n\nB=2\n\nC=4\n",
		},
		{
			"multi-line value moves intact",
			"A=1\nB=2\nA='x\ny'\n",
			"A=1\nA='x\ny'\nB=2\n",
		},
		{
			"already grouped is unchanged",
			"# a\nA=1\n# A=2\n\nB=3\n",
			"# a\nA=1\n# A=2\n\nB=3\n",
		},
	}
	for _, c := range cases {
		f, err := Parse([]byte(c.src))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		changed := f.Format()
		if got := string(f.Bytes()); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if changed != (c.src != c.want) {
			t.Errorf("%s: changed=%v", c.name, changed)
		}
		// Idempotent.
		if f.Format() {
			t.Errorf("%s: second Format reported a change", c.name)
		}
		// Values and descriptions survive.
		again, _ := Parse(f.Bytes())
		orig, _ := Parse([]byte(c.src))
		for _, e := range orig.Entries() {
			if v, _ := again.Lookup(e.Key); v != e.Value {
				t.Errorf("%s: value of %s changed: %q -> %q", c.name, e.Key, e.Value, v)
			}
		}
	}
}

func TestOccurrences(t *testing.T) {
	f, _ := Parse([]byte("# first\nA=1\nB=x\n\n# second\n# A='two\n# lines'\nA=3\n"))
	occ := f.Occurrences("A")
	want := []Occurrence{
		{Line: 2, Value: "1", Description: "first"},
		{Line: 6, Value: "two\nlines", Description: "second", Disabled: true},
		{Line: 8, Value: "3"},
	}
	if len(occ) != len(want) {
		t.Fatalf("got %+v", occ)
	}
	for i := range want {
		if occ[i] != want[i] {
			t.Errorf("occurrence %d: got %+v want %+v", i, occ[i], want[i])
		}
	}
	if len(f.Occurrences("ZZZ")) != 0 {
		t.Error("missing key should have no occurrences")
	}
}

func TestAllOccurrences(t *testing.T) {
	f, _ := Parse([]byte("# header\nA=1\nB='x\ny'\n# staging\n# A=2\nA=3\n"))
	got := f.AllOccurrences()
	want := []Row{
		{Line: 2, Key: "A", Value: "1", Description: "header"},
		{Line: 3, Key: "B", Value: "x\ny"},
		{Line: 6, Key: "A", Value: "2", Description: "staging", Disabled: true},
		{Line: 7, Key: "A", Value: "3"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: got %+v want %+v", i, got[i], want[i])
		}
	}
	// The effective view still collapses to the last active value.
	if e := f.Entries(); len(e) != 2 || e[0].Value != "3" {
		t.Errorf("Entries: %+v", e)
	}
}
