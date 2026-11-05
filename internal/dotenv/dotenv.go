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
	// Disabled marks a commented-out entry such as "# KEY=value". It is not
	// an active value, but it is tracked so it can be re-enabled and so the
	// comment block above it stays attached to it.
	Disabled bool
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
		if trimmed == "" {
			f.Lines = append(f.Lines, Line{Raw: line})
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			// A comment whose content is itself a KEY=value line is a
			// disabled entry. Gather the run of comment lines so a
			// multi-line quoted value that was commented out still parses.
			j := i
			var stripped []string
			for j < len(raw) && strings.HasPrefix(strings.TrimSpace(raw[j]), "#") {
				stripped = append(stripped, uncomment(raw[j]))
				j++
			}
			if l, consumed, err := parseEntry(stripped); err == nil && l.Entry {
				l.Entry, l.Disabled = false, true
				l.Raw = strings.Join(raw[i:i+1+consumed], "\n")
				f.Lines = append(f.Lines, l)
				i += consumed
				continue
			}
			f.Lines = append(f.Lines, Line{Raw: line})
			continue
		}
		l, consumed, err := parseEntry(raw[i:])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		if !l.Entry {
			f.Lines = append(f.Lines, Line{Raw: line})
			continue
		}
		l.Raw = strings.Join(raw[i:i+1+consumed], "\n")
		f.Lines = append(f.Lines, l)
		i += consumed
	}
	return f, nil
}

// uncomment strips the leading "#" and at most one following space from a
// comment line, preserving indentation.
func uncomment(line string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	rest := strings.TrimPrefix(strings.TrimLeft(line, " \t"), "#")
	return indent + strings.TrimPrefix(rest, " ")
}

// parseEntry parses a KEY=value entry beginning at lines[0], which may
// continue onto following lines inside quotes. It returns the entry (with
// Raw unset), the number of extra lines consumed, and an error for an
// unterminated quote. A line that is not an entry yields Entry=false.
func parseEntry(lines []string) (Line, int, error) {
	trimmed := strings.TrimSpace(lines[0])
	eq := strings.IndexByte(trimmed, '=')
	if eq < 0 {
		return Line{}, 0, nil
	}
	keyPart := strings.TrimSpace(trimmed[:eq])
	export := false
	if rest, ok := strings.CutPrefix(keyPart, "export"); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
		export = true
		keyPart = strings.TrimSpace(rest)
	}
	if !ValidKey(keyPart) {
		return Line{}, 0, nil
	}
	valPart := strings.TrimLeft(trimmed[eq+1:], " \t")

	var value, comment string
	consumed := 0
	switch {
	case strings.HasPrefix(valPart, `"`):
		v, extra, rest, err := parseDoubleQuoted(valPart[1:], lines[1:])
		if err != nil {
			return Line{}, 0, err
		}
		value, consumed, comment = v, extra, trailingComment(rest)
	case strings.HasPrefix(valPart, `'`):
		v, extra, rest, err := parseSingleQuoted(valPart[1:], lines[1:])
		if err != nil {
			return Line{}, 0, err
		}
		value, consumed, comment = v, extra, trailingComment(rest)
	default:
		value, comment = splitInlineComment(valPart)
	}
	return Line{Key: keyPart, Value: value, Export: export, Entry: true, Comment: comment}, consumed, nil
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
// or appending a new line. If the key exists only as a disabled entry, that
// entry is re-enabled in place.
func (f *File) Set(key, value string) { f.SetState(key, value, false) }

// SetState is Set with control over whether the resulting occurrence is
// active or disabled (commented out).
func (f *File) SetState(key, value string, disabled bool) {
	i := f.lastIndex(key)
	if i < 0 {
		f.Lines = append(f.Lines, Line{Key: key})
		i = len(f.Lines) - 1
	}
	l := &f.Lines[i]
	l.Value = value
	l.Entry, l.Disabled = true, false
	l.Raw = render(*l)
	if disabled {
		l.Entry, l.Disabled = false, true
		l.Raw = "# " + l.Raw
	}
}

// Unset removes every occurrence of key along with the comment block directly
// above each one. A blank line that would be left doubled, or left dangling at
// the end of the file, is removed too. It reports whether anything was removed.
func (f *File) Unset(key string) bool {
	removed := false
	for i := len(f.Lines) - 1; i >= 0; i-- {
		if !(f.Lines[i].Entry || f.Lines[i].Disabled) || f.Lines[i].Key != key {
			continue
		}
		removed = true
		i = f.removeAt(i)
	}
	return removed
}

// removeAt deletes the entry at Lines[i] with its comment block, collapsing a
// blank line that would be left doubled or dangling. It returns the index
// where the removed range began.
func (f *File) removeAt(i int) int {
	start := f.commentBlockStart(i)
	end := i + 1
	prevBlank := start > 0 && strings.TrimSpace(f.Lines[start-1].Raw) == ""
	nextBlank := end >= len(f.Lines) || strings.TrimSpace(f.Lines[end].Raw) == ""
	if prevBlank && nextBlank {
		start--
	}
	f.Lines = append(f.Lines[:start:start], f.Lines[end:]...)
	return start
}

// Remove deletes one occurrence with its comment block.
func (f *File) Remove(o Occurrence) { f.removeAt(o.index) }

func (f *File) disableAt(i int) {
	l := &f.Lines[i]
	if l.Disabled {
		return
	}
	parts := strings.Split(l.Raw, "\n")
	for j, p := range parts {
		parts[j] = "# " + p
	}
	l.Raw = strings.Join(parts, "\n")
	l.Entry, l.Disabled = false, true
}

func (f *File) enableAt(i int) {
	l := &f.Lines[i]
	if l.Entry {
		return
	}
	parts := strings.Split(l.Raw, "\n")
	for j, p := range parts {
		parts[j] = uncomment(p)
	}
	l.Raw = strings.Join(parts, "\n")
	l.Entry, l.Disabled = true, false
}

// EnableOccurrence uncomments one occurrence, leaving its siblings as they are.
func (f *File) EnableOccurrence(o Occurrence) { f.enableAt(o.index) }

// DisableOccurrence comments out one occurrence, leaving its siblings as they are.
func (f *File) DisableOccurrence(o Occurrence) { f.disableAt(o.index) }

// Use makes o the only active occurrence of its key: it is enabled and every
// other occurrence of the key is disabled.
func (f *File) Use(o Occurrence) {
	key := f.Lines[o.index].Key
	for i := range f.Lines {
		if (f.Lines[i].Entry || f.Lines[i].Disabled) && f.Lines[i].Key == key {
			if i == o.index {
				f.enableAt(i)
			} else {
				f.disableAt(i)
			}
		}
	}
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

// Described is a key with its position and the comment block above it.
type Described struct {
	Line        int // 1-based line number of the entry
	Key         string
	Description string // "" when there is no comment block
	Disabled    bool   // the entry is commented out
}

func isComment(l Line) bool {
	return !l.Entry && !l.Disabled && strings.HasPrefix(strings.TrimSpace(l.Raw), "#")
}

// commentBlockStart returns the index of the first line of the contiguous
// comment block directly above Lines[i], or i when there is none.
func (f *File) commentBlockStart(i int) int {
	start := i
	for start > 0 && isComment(f.Lines[start-1]) {
		start--
	}
	return start
}

// lastIndex returns the index of the last active entry for key, falling back
// to the last disabled entry, or -1.
func (f *File) lastIndex(key string) int {
	disabled := -1
	for i := len(f.Lines) - 1; i >= 0; i-- {
		l := f.Lines[i]
		if l.Key != key {
			continue
		}
		if l.Entry {
			return i
		}
		if l.Disabled && disabled < 0 {
			disabled = i
		}
	}
	return disabled
}

func (f *File) descriptionAt(i int) string {
	start := f.commentBlockStart(i)
	if start == i {
		return ""
	}
	parts := make([]string, 0, i-start)
	for _, l := range f.Lines[start:i] {
		t := strings.TrimPrefix(strings.TrimSpace(l.Raw), "#")
		parts = append(parts, strings.TrimPrefix(t, " "))
	}
	return strings.Join(parts, "\n")
}

// Explain returns the comment block directly above the last occurrence of
// key, with comment markers removed. ok is false when the key is absent.
func (f *File) Explain(key string) (desc string, ok bool) {
	i := f.lastIndex(key)
	if i < 0 {
		return "", false
	}
	return f.descriptionAt(i), true
}

// ExplainAll describes every key in file order. Duplicate keys report their
// last occurrence.
func (f *File) ExplainAll() []Described {
	idx := map[string]int{}
	var out []Described
	lineNo := 1
	for i, l := range f.Lines {
		if l.Entry || l.Disabled {
			d := Described{Line: lineNo, Key: l.Key, Description: f.descriptionAt(i), Disabled: l.Disabled}
			if j, seen := idx[l.Key]; seen {
				out[j] = d
			} else {
				idx[l.Key] = len(out)
				out = append(out, d)
			}
		}
		lineNo += strings.Count(l.Raw, "\n") + 1
	}
	return out
}

// SetComment replaces the comment block directly above key with text, one
// "# " line per line of text. An empty text removes the block. It reports
// whether the key exists.
func (f *File) SetComment(key, text string) bool {
	i := f.lastIndex(key)
	if i < 0 {
		return false
	}
	f.setCommentAt(i, text, true)
	return true
}

// setCommentAt replaces the comment block above Lines[i] with text. When
// separate is set, a fresh block directly below a non-blank line is preceded
// by a blank line so it does not read as part of the line above.
func (f *File) setCommentAt(i int, text string, separate bool) {
	start := f.commentBlockStart(i)
	var block []Line
	if strings.TrimSpace(text) != "" {
		for _, t := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
			t = strings.TrimRight(t, " \t")
			if t == "" {
				block = append(block, Line{Raw: "#"})
			} else {
				block = append(block, Line{Raw: "# " + t})
			}
		}
		// Separate a fresh block from a preceding entry with a blank line so
		// the comment does not read as belonging to the line above.
		if separate && start == i && start > 0 && strings.TrimSpace(f.Lines[start-1].Raw) != "" {
			block = append([]Line{{Raw: ""}}, block...)
		}
	}
	lines := make([]Line, 0, len(f.Lines)-(i-start)+len(block))
	lines = append(lines, f.Lines[:start]...)
	lines = append(lines, block...)
	lines = append(lines, f.Lines[i:]...)
	f.Lines = lines
}

// Disable comments out every active entry for key, preserving its text so it
// can be re-enabled. It reports whether the key exists in any state.
func (f *File) Disable(key string) bool {
	found := false
	for i := range f.Lines {
		l := &f.Lines[i]
		if l.Key != key || !(l.Entry || l.Disabled) {
			continue
		}
		found = true
		f.disableAt(i)
	}
	return found
}

// Enable uncomments every disabled entry for key. It reports whether the key
// exists in any state.
func (f *File) Enable(key string) bool {
	found := false
	for i := range f.Lines {
		l := &f.Lines[i]
		if l.Key != key || !(l.Entry || l.Disabled) {
			continue
		}
		found = true
		f.enableAt(i)
	}
	return found
}

// DisabledEntries returns disabled entries in file order, duplicates
// collapsed to their last occurrence.
func (f *File) DisabledEntries() []Line {
	idx := map[string]int{}
	var out []Line
	for _, l := range f.Lines {
		if !l.Disabled {
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

// Format groups every occurrence of a key, active or disabled, together at
// the position of its first occurrence. Occurrences keep their relative order
// and each keeps the comment block directly above it. Blank lines left
// doubled or dangling by a move are removed. It reports whether anything
// changed.
func (f *File) Format() bool {
	before := f.Bytes()

	// Segment into units: a key unit spans an entry's comment block and the
	// entry itself; everything else is a single filler line.
	type unit struct {
		lines []Line
		key   string // "" for filler
	}
	var units []unit
	for i := 0; i < len(f.Lines); i++ {
		// Find the next entry; everything before its block is filler.
		j := i
		for j < len(f.Lines) && !(f.Lines[j].Entry || f.Lines[j].Disabled) {
			j++
		}
		if j == len(f.Lines) {
			for _, l := range f.Lines[i:] {
				units = append(units, unit{lines: []Line{l}})
			}
			break
		}
		start := f.commentBlockStart(j)
		for _, l := range f.Lines[i:start] {
			units = append(units, unit{lines: []Line{l}})
		}
		units = append(units, unit{lines: f.Lines[start : j+1], key: f.Lines[j].Key})
		i = j
	}

	byKey := map[string][]unit{}
	for _, u := range units {
		if u.key != "" {
			byKey[u.key] = append(byKey[u.key], u)
		}
	}

	isBlank := func(u unit) bool {
		return u.key == "" && len(u.lines) == 1 && strings.TrimSpace(u.lines[0].Raw) == ""
	}

	var out []Line
	placed := map[string]bool{}
	skip := make([]bool, len(units))
	for k, u := range units {
		if skip[k] {
			continue
		}
		if u.key == "" {
			out = append(out, u.lines...)
			continue
		}
		if placed[u.key] {
			// This occurrence has been moved up. Avoid leaving a doubled
			// blank line (or a trailing one) where it used to be.
			prevBlank := len(out) > 0 && strings.TrimSpace(out[len(out)-1].Raw) == ""
			next := k + 1
			for next < len(units) && skip[next] {
				next++
			}
			nextBlank := next >= len(units) || isBlank(units[next])
			if prevBlank && nextBlank {
				if next < len(units) {
					skip[next] = true
				} else {
					out = out[:len(out)-1]
				}
			}
			continue
		}
		placed[u.key] = true
		for _, occ := range byKey[u.key] {
			out = append(out, occ.lines...)
		}
	}
	f.Lines = out
	return !bytes.Equal(before, f.Bytes())
}

// Occurrence is one appearance of a key in the file.
type Occurrence struct {
	Line        int // 1-based line number of the entry
	Value       string
	Description string
	Disabled    bool
	index       int // position in File.Lines; valid until the file is mutated
}

// Occurrences returns every appearance of key, active or disabled, in file
// order.
func (f *File) Occurrences(key string) []Occurrence {
	var out []Occurrence
	lineNo := 1
	for i, l := range f.Lines {
		if (l.Entry || l.Disabled) && l.Key == key {
			out = append(out, Occurrence{Line: lineNo, Value: l.Value, Description: f.descriptionAt(i), Disabled: l.Disabled, index: i})
		}
		lineNo += strings.Count(l.Raw, "\n") + 1
	}
	return out
}

// Row is one occurrence of any key, used for a file-order listing.
type Row struct {
	Line        int // 1-based line number of the entry
	Key         string
	Value       string
	Description string // comment block above the entry, "" if none
	Disabled    bool
}

// AllOccurrences returns every entry in the file, active or disabled, in
// file order, with duplicates kept.
func (f *File) AllOccurrences() []Row {
	var out []Row
	lineNo := 1
	for i, l := range f.Lines {
		if l.Entry || l.Disabled {
			out = append(out, Row{Line: lineNo, Key: l.Key, Value: l.Value, Description: f.descriptionAt(i), Disabled: l.Disabled})
		}
		lineNo += strings.Count(l.Raw, "\n") + 1
	}
	return out
}

// Append adds a new occurrence of key directly after its last existing
// occurrence (active or disabled), or at the end of the file when the key is
// new. The new occurrence is active unless disabled is set; when active it is
// the last occurrence and so becomes the effective value. A non-empty comment
// becomes the block above the new occurrence.
func (f *File) Append(key, value string, disabled bool, comment string) {
	l := Line{Key: key, Value: value, Entry: true}
	l.Raw = render(l)
	if disabled {
		l.Entry, l.Disabled = false, true
		l.Raw = "# " + l.Raw
	}

	at := len(f.Lines)
	grouped := false
	for i := len(f.Lines) - 1; i >= 0; i-- {
		if (f.Lines[i].Entry || f.Lines[i].Disabled) && f.Lines[i].Key == key {
			at, grouped = i+1, true
			break
		}
	}
	f.Lines = append(f.Lines[:at], append([]Line{l}, f.Lines[at:]...)...)
	if strings.TrimSpace(comment) != "" {
		// Inside a group the block sits directly under the previous
		// occurrence; a new key at the end of the file gets a separating blank.
		f.setCommentAt(at, comment, !grouped)
	}
}
