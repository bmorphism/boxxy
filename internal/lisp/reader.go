//go:build darwin

package lisp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// Value represents a Lisp value
type Value interface {
	String() string
}

// Nil represents nil
type Nil struct{}

func (Nil) String() string { return "nil" }

// Bool represents a boolean
type Bool bool

func (b Bool) String() string {
	if b {
		return "true"
	}
	return "false"
}

// Int represents an integer
type Int int64

func (i Int) String() string { return strconv.FormatInt(int64(i), 10) }

// Float represents a float
type Float float64

func (f Float) String() string { return strconv.FormatFloat(float64(f), 'f', -1, 64) }

// String represents a string
type String string

func (s String) String() string { return fmt.Sprintf("%q", string(s)) }

// Symbol represents a symbol
type Symbol string

func (s Symbol) String() string { return string(s) }

// Keyword represents a keyword
type Keyword string

func (k Keyword) String() string { return ":" + string(k) }

// List represents a list
type List []Value

func (l List) String() string {
	parts := make([]string, len(l))
	for i, v := range l {
		parts[i] = v.String()
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// Vector represents a vector
type Vector []Value

func (v Vector) String() string {
	parts := make([]string, len(v))
	for i, val := range v {
		parts[i] = val.String()
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// HashMap represents a hash map
type HashMap map[Value]Value

func (h HashMap) String() string {
	parts := make([]string, 0, len(h)*2)
	for k, v := range h {
		parts = append(parts, k.String(), v.String())
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// Fn represents a native function
type Fn struct {
	Name string
	Func func([]Value) Value
}

func (f *Fn) String() string { return fmt.Sprintf("#<fn:%s>", f.Name) }

// ExternalValue wraps Go values
type ExternalValue struct {
	Value interface{}
	Type  string
}

func (e *ExternalValue) String() string {
	return fmt.Sprintf("#<%s>", e.Type)
}

// Reader reads Lisp expressions
type Reader struct {
	reader *bufio.Reader
	line   int
	col    int
}

// NewReader creates a new reader
func NewReader(r io.Reader) *Reader {
	return &Reader{
		reader: bufio.NewReader(r),
		line:   1,
		col:    0,
	}
}

func (r *Reader) read() (rune, error) {
	ch, _, err := r.reader.ReadRune()
	if err != nil {
		return 0, err
	}
	if ch == '\n' {
		r.line++
		r.col = 0
	} else {
		r.col++
	}
	return ch, nil
}

func (r *Reader) unread() {
	r.reader.UnreadRune()
	r.col--
}

func (r *Reader) peek() (rune, error) {
	ch, err := r.read()
	if err != nil {
		return 0, err
	}
	r.unread()
	return ch, nil
}

func (r *Reader) skipWhitespace() error {
	for {
		ch, err := r.read()
		if err != nil {
			return err
		}
		if ch == ';' {
			// Skip comment
			for {
				ch, err := r.read()
				if err != nil {
					return err
				}
				if ch == '\n' {
					break
				}
			}
			continue
		}
		if !unicode.IsSpace(ch) && ch != ',' {
			r.unread()
			return nil
		}
	}
}

func isTerminator(ch rune) bool {
	return unicode.IsSpace(ch) || ch == '(' || ch == ')' || ch == '[' || ch == ']' ||
		ch == '{' || ch == '}' || ch == '"' || ch == ';' || ch == ','
}

// Read reads the next expression
func (r *Reader) Read() (Value, error) {
	if err := r.skipWhitespace(); err != nil {
		return nil, err
	}

	ch, err := r.read()
	if err != nil {
		return nil, err
	}

	switch ch {
	case '(':
		return r.readList(')')
	case '[':
		return r.readVector()
	case '{':
		return r.readHashMap()
	case '"':
		return r.readString()
	case ':':
		return r.readKeyword()
	case '\'':
		val, err := r.Read()
		if err != nil {
			return nil, err
		}
		return List{Symbol("quote"), val}, nil
	case '#':
		// Check for shebang
		next, err := r.peek()
		if err == nil && next == '!' {
			// Skip shebang line
			for {
				ch, err := r.read()
				if err != nil || ch == '\n' {
					break
				}
			}
			return r.Read()
		}
		r.unread()
		return r.readAtom()
	default:
		r.unread()
		return r.readAtom()
	}
}

func (r *Reader) readList(endCh rune) (List, error) {
	var list List
	for {
		if err := r.skipWhitespace(); err != nil {
			return nil, err
		}
		ch, err := r.peek()
		if err != nil {
			return nil, fmt.Errorf("unexpected EOF in list")
		}
		if ch == endCh {
			r.read()
			return list, nil
		}
		val, err := r.Read()
		if err != nil {
			return nil, err
		}
		list = append(list, val)
	}
}

func (r *Reader) readVector() (Vector, error) {
	list, err := r.readList(']')
	if err != nil {
		return nil, err
	}
	return Vector(list), nil
}

func (r *Reader) readHashMap() (HashMap, error) {
	list, err := r.readList('}')
	if err != nil {
		return nil, err
	}
	if len(list)%2 != 0 {
		return nil, fmt.Errorf("hash map must have even number of elements")
	}
	m := make(HashMap)
	for i := 0; i < len(list); i += 2 {
		m[list[i]] = list[i+1]
	}
	return m, nil
}

func (r *Reader) readString() (String, error) {
	var buf strings.Builder
	for {
		ch, err := r.read()
		if err != nil {
			return "", fmt.Errorf("unexpected EOF in string")
		}
		if ch == '"' {
			return String(buf.String()), nil
		}
		if ch == '\\' {
			ch, err = r.read()
			if err != nil {
				return "", err
			}
			switch ch {
			case 'n':
				buf.WriteRune('\n')
			case 't':
				buf.WriteRune('\t')
			case 'r':
				buf.WriteRune('\r')
			case '"':
				buf.WriteRune('"')
			case '\\':
				buf.WriteRune('\\')
			default:
				buf.WriteRune('\\')
				buf.WriteRune(ch)
			}
		} else {
			buf.WriteRune(ch)
		}
	}
}

func (r *Reader) readKeyword() (Keyword, error) {
	var buf strings.Builder
	for {
		ch, err := r.read()
		if err != nil {
			break
		}
		if isTerminator(ch) {
			r.unread()
			break
		}
		buf.WriteRune(ch)
	}
	return Keyword(buf.String()), nil
}

func (r *Reader) readAtom() (Value, error) {
	var buf strings.Builder
	for {
		ch, err := r.read()
		if err != nil {
			break
		}
		if isTerminator(ch) {
			r.unread()
			break
		}
		buf.WriteRune(ch)
	}

	s := buf.String()
	if s == "" {
		return nil, io.EOF
	}

	// Check for special atoms
	switch s {
	case "nil":
		return Nil{}, nil
	case "true":
		return Bool(true), nil
	case "false":
		return Bool(false), nil
	}

	// Try to parse as int
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return Int(i), nil
	}

	// Try to parse as float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return Float(f), nil
	}

	return Symbol(s), nil
}

// ReadAll reads all expressions from the reader
func (r *Reader) ReadAll() ([]Value, error) {
	var values []Value
	for {
		val, err := r.Read()
		if err == io.EOF {
			return values, nil
		}
		if err != nil {
			return nil, err
		}
		values = append(values, val)
	}
}
