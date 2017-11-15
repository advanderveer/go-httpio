package encoding_test

import (
	"bytes"
	"testing"

	"github.com/advanderveer/go-httpio/encoding"
	"github.com/gorilla/schema"
)

func TestEncodingStd(t *testing.T) {
	var e encoding.Encoding
	e = &encoding.JSON{}
	if e.MimeType() != "application/json" {
		t.Fatal("should be application json")
	}

	e = &encoding.XML{}
	if e.MimeType() != "application/xml" {
		t.Fatal("should be application xml")
	}
}

func TestEncodingForm(t *testing.T) {
	var e encoding.Encoding
	e = encoding.NewFormEncoding(schema.NewEncoder(), schema.NewDecoder())
	if e.MimeType() != "application/x-www-form-urlencoded" {
		t.Fatal("mime type should be application/x-www-form-urlencoded")
	}

	t.Run("decoding", func(t *testing.T) {
		buf := bytes.NewBufferString(`foo=bar&bar=foo`)
		dec := e.Decoder(buf)
		v := struct {
			Foo string
			Bar string
		}{}
		err := dec.Decode(&v)
		if err != nil {
			t.Fatal(err)
		}

		if v.Bar != "foo" && v.Foo != "bar" {
			t.Fatal("decoding failed")
		}
	})

	t.Run("decoding", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := e.Encoder(buf)
		v := struct {
			Foo string `schema:"foo"`
			Bar string `schema:"bar"`
		}{"bar", "foo"}
		err := enc.Encode(v)
		if err != nil {
			t.Fatal(err)
		}

		if buf.String() != `bar=foo&foo=bar` {
			t.Fatal("encoding failed, got: %v", buf.String())
		}
	})

}
