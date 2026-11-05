// Package dotenv parses and edits dotenv files while preserving comments,
// blank lines, ordering, and unrelated formatting.
package dotenv

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var keyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// Line is one logical line of a dotenv file. Non-entry lines (comments,
// blanks, unparseable text) carry only Raw and are written back verbatim.
type Line struct {
	Raw    string // original text, may contain newlines for multi-line quoted values
	Key    string
	Value  string
	Export bool
	Entry  bool
	// Comment is a trailing inline comment (including the leading "#"),
	// preserved when the entry is rewritten.
	Comment string
}

// File is an ordered dotenv document.
type File struct {
	Lines []Line
	// Newline is the line terminator detected in the source ("\n" or "\r\n").
	Newline string
	// TrailingNewline records whether the source ended with a newline.
	TrailingNewline bool
}

// ValidKey reports whether k is an acceptable variable name.
func ValidKey(k string) bool { return keyRe.MatchString(k) }

// Parse parses dotenv content.
func Parse(data []byte) (*File, error) {
	f := &File{Newline: "\n", TrailingNewline: true}
	if bytes.Contains(data, []byte("\r\n")) {
		f.Newline = "\r\n"
	}
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	if src == "" {
		return f, nil
	}
	f.TrailingNewline = strings.HasSuffix(src, "\n")
	src = strings.TrimSuffix(src, "\n")
	raw := strings.Split(src, "\n")

	for i := 0; i < len(raw); i++ {
		line := raw[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			f.Lines = append(f.Lines, Line{Raw: line})
			continue
		}
		eq := strings.IndexByte(trimmed, '=')
		if eq < 0 {
			f.Lines = append(f.Lines, Line{Raw: line})
			continue
		}
		keyPart := strings.TrimSpace(trimmed[:eq])
		export := false
		if rest, ok := strings.CutPrefix(keyPart, "export"); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
			if rest != "" {
				export = true
				keyPart = strings.TrimSpace(rest)
			}
		}
		if !ValidKey(keyPart) {
			f.Lines = append(f.Lines, Line{Raw: line})
			continue
		}
		valPart := strings.TrimLeft(trimmed[eq+1:], " \t")

		var value, comment string
		consumed := 0
		switch {
		case strings.HasPrefix(valPart, `"`):
			v, extra, rest, err := parseDoubleQuoted(valPart[1:], raw[i+1:])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			value, consumed, comment = v, extra, trailingComment(rest)
		case strings.HasPrefix(valPart, `'`):
			v, extra, rest, err := parseSingleQuoted(valPart[1:], raw[i+1:])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			value, consumed, comment = v, extra, trailingComment(rest)
		default:
			value, comment = splitInlineComment(valPart)
		}
		rawText := strings.Join(raw[i:i+1+consumed], "\n")
		i += consumed
		f.Lines = append(f.Lines, Line{Raw: rawText, Key: keyPart, Value: value, Export: export, Entry: true, Comment: comment})
	}
	return f, nil
}

// splitInlineComment separates an unquoted value from a trailing " # comment".
func splitInlineComment(s string) (value, comment string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return strings.TrimRight(s[:i], " \t"), s[i:]
		}
	}
	return strings.TrimRight(s, " \t"), ""
}

// trailingComment returns the comment, if any, in the text after a closing quote.
func trailingComment(rest string) string {
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "#") {
		return rest
	}
	return ""
}

// parseDoubleQuoted reads a double-quoted value starting after the opening
// quote. It may span following lines; it returns the value, the number of
// extra lines consumed, and any text following the closing quote.
func parseDoubleQuoted(first string, more []string) (string, int, string, error) {
	var out strings.Builder
	cur := first
	extra := 0
	for {
		for i := 0; i < len(cur); i++ {
			c := cur[i]
			switch c {
			case '\\':
				if i+1 >= len(cur) {
					out.WriteByte('\\')
					continue
				}
				i++
				switch cur[i] {
				case 'n':
					out.WriteByte('\n')
				case 't':
					out.WriteByte('\t')
				case 'r':
					out.WriteByte('\r')
				case '"', '\\', '$':
					out.WriteByte(cur[i])
				default:
					out.WriteByte('\\')
					out.WriteByte(cur[i])
				}
			case '"':
				return out.String(), extra, cur[i+1:], nil
			default:
				out.WriteByte(c)
			}
		}
		if extra >= len(more) {
			return "", 0, "", errors.New("unterminated double-quoted value")
		}
		out.WriteByte('\n')
		cur = more[extra]
		extra++
	}
}

func parseSingleQuoted(first string, more []string) (string, int, string, error) {
	var out strings.Builder
	cur := first
	extra := 0
	for {
		if i := strings.IndexByte(cur, '\''); i >= 0 {
			out.WriteString(cur[:i])
			return out.String(), extra, cur[i+1:], nil
		}
		out.WriteString(cur)
		if extra >= len(more) {
			return "", 0, "", errors.New("unterminated single-quoted value")
		}
		out.WriteByte('\n')
		cur = more[extra]
		extra++
	}
}

// Quote renders v in the simplest form that round-trips through Parse.
func Quote(v string) string {
	if v == "" {
		return `""`
	}
	if !strings.ContainsAny(v, " \t\n\r\"'#$\\`") && strings.TrimSpace(v) == v {
		return v
	}
	if !strings.ContainsAny(v, "'\n\r") {
		return "'" + v + "'"
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range v {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '$':
			b.WriteString(`\$`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func render(l Line) string {
	if !l.Entry {
		return l.Raw
	}
	prefix := ""
	if l.Export {
		prefix = "export "
	}
	line := prefix + l.Key + "=" + Quote(l.Value)
	if l.Comment != "" {
		line += " " + l.Comment
	}
	return line
}

// Lookup returns the value of key. When the key appears more than once the
// last occurrence wins, matching how shells and most loaders behave.
func (f *File) Lookup(key string) (string, bool) {
	for i := len(f.Lines) - 1; i >= 0; i-- {
		if f.Lines[i].Entry && f.Lines[i].Key == key {
			return f.Lines[i].Value, true
		}
	}
	return "", false
}

// Entries returns all key/value entries in file order, with duplicates
// collapsed to their last occurrence (positioned at the first occurrence).
func (f *File) Entries() []Line {
	idx := map[string]int{}
	var out []Line
	for _, l := range f.Lines {
		if !l.Entry {
			continue
		}
		if i, ok := idx[l.Key]; ok {
			out[i] = l
			continue
		}
		idx[l.Key] = len(out)
		out = append(out, l)
	}
	return out
}

// Set assigns value to key, rewriting the last existing occurrence in place
// or appending a new line.
func (f *File) Set(key, value string) {
	for i := len(f.Lines) - 1; i >= 0; i-- {
		if f.Lines[i].Entry && f.Lines[i].Key == key {
			f.Lines[i].Value = value
			f.Lines[i].Raw = render(f.Lines[i])
			return
		}
	}
	l := Line{Key: key, Value: value, Entry: true}
	l.Raw = render(l)
	f.Lines = append(f.Lines, l)
}

// Unset removes every occurrence of key. It reports whether anything was removed.
func (f *File) Unset(key string) bool {
	kept := f.Lines[:0]
	removed := false
	for _, l := range f.Lines {
		if l.Entry && l.Key == key {
			removed = true
			continue
		}
		kept = append(kept, l)
	}
	f.Lines = kept
	return removed
}

// Bytes serializes the file.
func (f *File) Bytes() []byte {
	var b strings.Builder
	for i, l := range f.Lines {
		b.WriteString(l.Raw)
		if i < len(f.Lines)-1 || f.TrailingNewline {
			b.WriteString("\n")
		}
	}
	s := b.String()
	if f.Newline != "\n" {
		s = strings.ReplaceAll(s, "\n", f.Newline)
	}
	return []byte(s)
}

// Load reads and parses path. A missing file yields an empty File and
// os.ErrNotExist so callers can decide whether that is an error.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{Newline: "\n", TrailingNewline: true}, err
		}
		return nil, err
	}
	return Parse(data)
}

// Save writes f to path atomically, preserving the existing file mode.
func Save(path string, f *File) error {
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(f.Bytes()); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
