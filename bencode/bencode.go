package bencode

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"unicode"
)

type BencodeParser struct {
    Source string
    Current int
}

func NewBencodeParser(source string) *BencodeParser {
    return &BencodeParser{
        Source: source,
        Current: 0,
    }
}

func (p *BencodeParser) expect(ch rune, msg string) error {
    // the caller should make sure b.Current is valid
    if p.peek() == ch {
        p.Current++
        return nil
    }

    return fmt.Errorf("%s", msg)
}

func (p *BencodeParser) peek() rune {
    // the caller should make sure b.Current is valid
    return rune(p.Source[p.Current])
}

func (p *BencodeParser) isEnd() bool {
    return p.Current >= len(p.Source)
}

func (p *BencodeParser) parseString() (str string, err error) {
    lengthStr := ""
    start := p.Current
    for !p.isEnd() {
        if p.peek() == ':' {
            lengthStr = p.Source[start: p.Current]
            p.Current++
            break
        }
        p.Current++
    }

    if lengthStr == "" {
        return "", fmt.Errorf("Invalid string format")
    }

    var length int
    if length, err = strconv.Atoi(lengthStr); err != nil {
        return
    }

    if length + p.Current > len(p.Source) {
        return "", fmt.Errorf("Invalid string format")
    }

    str = p.Source[p.Current : p.Current + length]
    p.Current += length
    return
}

func (p *BencodeParser) parseInt() (i int, err error) {
    if err = p.expect('i', "Expect 'i' at the beginning of int"); err != nil {
        return
    }

    start := p.Current
    for !p.isEnd() && p.peek() != 'e' {
        p.Current++
    }

    if err = p.expect('e', "Expect 'e' at the end of int"); err != nil {
        return
    }

    return strconv.Atoi(p.Source[start : p.Current - 1])
}

func (p *BencodeParser) parseList() (lst []any, err error) {
    if err = p.expect('l', "Expect 'l' at the beginning of list"); err != nil {
        return
    }

    var elem any
    lst = make([]any, 0)
    for !p.isEnd() && p.peek() != 'e' {
        if elem, err = p.Parse(); err != nil {
            return
        }
        lst = append(lst, elem)
    }

    if err = p.expect('e', "Expect 'e' at the end of list"); err != nil {
        return
    }

    return
}

func (p *BencodeParser) parseDict() (mp map[string]any, err error) {
    if err = p.expect('d', "Expect 'd' at the beginning of map"); err != nil {
        return
    }

    var val any
    var key string
    mp = make(map[string]any)
    for !p.isEnd() && p.peek() != 'e' {
        if key, err = p.parseString(); err != nil {
            return
        }
        if val, err = p.Parse(); err != nil {
            return
        }
        mp[key] = val
    }

    if err = p.expect('e', "Expect 'e' at the end of map"); err != nil {
        return
    }

    return
}

func (p *BencodeParser) Parse() (any, error) {
    if p.isEnd() {
        return nil, fmt.Errorf("End of input string")
    }

    switch {
    case unicode.IsDigit(p.peek()):
        return p.parseString()
    case p.peek() == 'i':
        return p.parseInt()
    case p.peek() == 'l':
        return p.parseList()
    case p.peek() == 'd':
        return p.parseDict()
    }

    return nil, fmt.Errorf("Unknown type: '%c'", p.peek())
}

func Unmarshal(str string, val any) error {
    if reflect.TypeOf(val).Kind() != reflect.Ptr {
        return fmt.Errorf("Expect the pointer value")
    }

    parser := NewBencodeParser(str)
    data, err := parser.Parse()
    if err != nil {
        return err
    }

    return unmarshal(data, val)
}

func unmarshal(data any, val any) error {
    dt := reflect.TypeOf(data)
    switch dt.Kind() {
    case reflect.Int:
        return unmarshalInt(data.(int), val)
    case reflect.String:
        return unmarshalString(data.(string), val)
    case reflect.Slice:
        return unmarshalList(data.([]any), val)
    case reflect.Map:
        return unmarshalDict(data.(map[string]any), val)
    }

    return fmt.Errorf("Unknown type: %s", dt)
}

func unmarshalInt(i int, val any) error {
    reflect.ValueOf(val).Elem().SetInt(int64(i))
    return nil
}

func unmarshalString(str string, val any) error {
    reflect.ValueOf(val).Elem().SetString(str)
    return nil
}

func unmarshalList(lst []any, val any) error {
    vv := reflect.ValueOf(val).Elem()
    if vv.Kind() != reflect.Slice {
        return fmt.Errorf("Expect the slice value")
    }

    if len(lst) == 0 {
        return nil
    }

    // allocate the memory
    slice := reflect.MakeSlice(vv.Type(), len(lst), len(lst))
    vv.Set(slice)

    for idx, data := range(lst) {
        entry := vv.Index(idx)
        if !entry.CanAddr() {
            continue  // Expect the pointer
        }

        if err := unmarshal(data, entry.Addr().Interface()); err != nil {
            return err
        }
    }

    return nil
}

func unmarshalDict(mp map[string]any, val any) error {
    vv := reflect.ValueOf(val).Elem()
    if vv.Kind() != reflect.Struct {
        return fmt.Errorf("Expect the struct value")
    }

    for i:=0; i<vv.NumField(); i++ {
        ft := vv.Type().Field(i)
        key := ft.Tag.Get("bencode")
        if key == "" || !ft.IsExported() {
            continue  // Ignore the private fields
        }

        data, ok := mp[key]
        if !ok {
            continue  // Ignore the fields without data
        }

        fv := vv.Field(i)
        if !fv.CanAddr() {
            continue  // Expect the pointer
        }

        if err := unmarshal(data, fv.Addr().Interface()); err != nil {
            return err
        }
    }

    return nil
}

func Marshal(w io.Writer, val any) error {
    vt := reflect.TypeOf(val)
    if vt.Kind() == reflect.Ptr {
        return fmt.Errorf("Expect the value, not a pointer")
    }

    switch vt.Kind() {
    case reflect.Int:
        return marshalInt(w, val)
    case reflect.String:
        return marshalString(w, val)
    case reflect.Slice:
        return marshalList(w, val)
    case reflect.Struct:
        return marshalStruct(w, val)
    }

    return fmt.Errorf("Unknown type: %s", vt)
}

func marshalInt(w io.Writer, val any) error {
    w.Write([]byte{'i'})
    w.Write([]byte(strconv.Itoa(val.(int))))
    w.Write([]byte{'e'})

    return nil
}

func marshalString(w io.Writer, val any) error {
    str := val.(string)
    w.Write([]byte(strconv.Itoa(len(str))))
    w.Write([]byte{':'})
    w.Write([]byte(str))

    return nil
}

func marshalList(w io.Writer, val any) error {
    w.Write([]byte{'l'})

    vv := reflect.ValueOf(val)
    for i:=0; i<vv.Len(); i++ {
        if err := Marshal(w, vv.Index(i).Interface()); err != nil {
            return err
        }
    }

    w.Write([]byte{'e'})
    return nil
}

// Notes: the keys are sorted in lexicographical order
func marshalStruct(w io.Writer, val any) error {
    w.Write([]byte{'d'})

    vv := reflect.ValueOf(val)

    keys := make([]struct{
        idx int;
        key string
    }, 0, vv.NumField())

    for i:=0; i<vv.NumField(); i++ {
        ft := vv.Type().Field(i)
        key := ft.Tag.Get("bencode")
        if key == "" || !ft.IsExported() {
            continue  // Ignore the private fields
        }

        keys = append(keys, struct{idx int; key string}{idx: i, key: key})
    }

    sort.Slice(keys, func (i, j int) bool {
        return keys[i].key < keys[j].key
    })

    for _, key := range(keys) {
        if err := marshalString(w, key.key); err != nil {
            return err
        }
        if err := Marshal(w, vv.Field(key.idx).Interface()); err != nil {
            return err
        }
    }

    w.Write([]byte{'e'})
    return nil
}
