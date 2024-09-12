package bencode

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalString(t *testing.T) {
    var str string
    data := "5:hello"

    assert.Nil(t, Unmarshal(data, &str))
    assert.Equal(t, str, "hello")
}

func TestUnmarshalInt(t *testing.T) {
    var i int
    data := "i52e"

    assert.Nil(t, Unmarshal(data, &i))
    assert.Equal(t, i, 52)
}

func TestUnmarshalSlice1(t *testing.T) {
    var slice []string
    data := "l1:11:21:3e"

    assert.Nil(t, Unmarshal(data, &slice))
    assert.Equal(t, slice, []string{"1", "2", "3"})
}

func TestUnmarshalSlice2(t *testing.T) {
    var slice []int
    data := "li1ei2ei3ee"

    assert.Nil(t, Unmarshal(data, &slice))
    assert.Equal(t, slice, []int{1, 2, 3})
}

type Foo struct {
    F1 int `bencode:"f1"`
    F2 string `bencode:"f2"`
}

type Bar struct {
    B1 Foo `bencode:"b1"`
    B2 string `bencode:"b2"`
    B3 int                      // tag is empty
    b4 string `bencode:"b4"`    // field is private
}

func TestUnmarshalStruct1(t *testing.T) {
    var foo Foo
    data := "d2:f1i100e2:f23:fooe"

    assert.Nil(t, Unmarshal(data, &foo))
    assert.Equal(t, foo.F1, 100)
    assert.Equal(t, foo.F2, "foo")
}

func TestUnmarshalStruct2(t *testing.T) {
    var bar Bar 
    data := "d2:b1d2:f1i100e2:f23:fooe2:b23:bar2:b3i10e2:b45:emptye"

    assert.Nil(t, Unmarshal(data, &bar))
    assert.Equal(t, bar.B1.F1, 100)
    assert.Equal(t, bar.B1.F2, "foo")
    
    assert.Equal(t, bar.B2, "bar")
    assert.Equal(t, bar.B3, 0)  // this field doesn't unmarshal
    assert.Equal(t, bar.b4, "") // this field doesn't unmarshal
}

type Foo1 struct {
    F1 []int
    F2 string
    F3 []Foo
}

// TODO

func TestMarshalString(t *testing.T) {
    str := "hello world"
    var buf bytes.Buffer

    assert.Nil(t, Marshal(&buf, str))
    assert.Equal(t, buf.String(), "11:hello world")
}

func TestMarshalInt(t *testing.T) {
    i := 101
    var buf bytes.Buffer

    assert.Nil(t, Marshal(&buf, i))
    assert.Equal(t, buf.String(), "i101e")
}

func TestMarshalList(t *testing.T) {
    slice := []int{1, 2, 3}
    var buf bytes.Buffer

    assert.Nil(t, Marshal(&buf, slice))
    assert.Equal(t, buf.String(), "li1ei2ei3ee")
}

func TestMarshalStruct1(t *testing.T) {
    foo := Foo{
        F1: 123,
        F2: "123",
    }
    var buf bytes.Buffer
    assert.Nil(t, Marshal(&buf, foo))
    assert.Equal(t, buf.String(), "d2:f1i123e2:f23:123e")
}

func TestMarshalStruct2(t *testing.T) {
    bar := Bar {
        B1: Foo {
            F1: 456,
            F2: "456",
        },
        B2: "bar",
        B3: 100,
        b4: "private",
    }
    var buf bytes.Buffer
    assert.Nil(t, Marshal(&buf, bar))
    assert.Equal(t, buf.String(), "d2:b1d2:f1i456e2:f23:456e2:b23:bare")
    // the B3 and b4 fields should not be marshal
}
