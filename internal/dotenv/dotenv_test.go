package dotenv

import (
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
