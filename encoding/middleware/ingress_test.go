package middleware_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"

	validator "gopkg.in/go-playground/validator.v9"

	"github.com/advanderveer/go-httpio/encoding"
	"github.com/advanderveer/go-httpio/encoding/middleware"
	"github.com/advanderveer/go-httpio/handling"
	"github.com/gorilla/schema"
)

var queryParse = func(next middleware.Transformer) middleware.Transformer {
	return middleware.TransFunc(func(a interface{}, r *http.Request, w http.ResponseWriter) error {
		vals := r.URL.Query()
		if len(vals) > 0 {
			dec := schema.NewDecoder()
			err := dec.Decode(a, vals)
			if err != nil {
				return err
			}
		}

		return next.Transform(a, r, w)
	})
}

var validateIngress = func(next middleware.Transformer) middleware.Transformer {
	return middleware.TransFunc(func(a interface{}, r *http.Request, w http.ResponseWriter) error {
		val := validator.New()
		err := val.Struct(a)
		if err != nil {
			return err
		}

		return next.Transform(a, r, w)
	})
}

type testInput struct {
	Name  string
	Image string `schema:"form-image" json:"json-image" validate:"ascii"`
}

func TestDefaultParse(t *testing.T) {
	for _, c := range []struct {
		Name     string
		Wares    []middleware.Transware
		Others   []middleware.DecoderFactory
		Method   string
		Path     string
		Body     string
		Headers  http.Header
		ExpInput *testInput //we expect the body to be parsed into the input like this
		ExpErr   error
	}{
		{
			Name:     "plain GET should not decode as it has no content",
			Method:   http.MethodGet,
			ExpInput: &testInput{Name: ""},
		},
		{
			Name:     "GET with query should not decode as no form decoder is configured",
			Method:   http.MethodGet,
			Path:     "?name=foo",
			ExpInput: &testInput{Name: ""},
		},
		{
			Name:     "POST with json should work",
			Method:   http.MethodPost,
			Headers:  http.Header{"Content-Type": []string{"application/json"}},
			Body:     `{"json-image": "bar3"}`,
			ExpInput: &testInput{Name: "", Image: "bar3"},
		},
		{
			Name:     "GET with query should decode using form middleware",
			Wares:    []middleware.Transware{queryParse},
			Method:   http.MethodGet,
			Path:     "?name=foo",
			ExpInput: &testInput{Name: "foo"},
		},
		{
			Name:     "POST with query should overwrite partially",
			Wares:    []middleware.Transware{queryParse},
			Method:   http.MethodPost,
			Path:     "?name=foo&form-image=overwritten",
			Headers:  http.Header{"Content-Type": []string{"application/json"}},
			Body:     `{"json-image": "bar"}`,
			ExpInput: &testInput{Name: "foo", Image: "overwritten"},
		},
		{
			Name:     "Early error middleware should cause error",
			Wares:    []middleware.Transware{earlyReturnErrWare},
			Method:   http.MethodGet,
			Path:     "?name=foo",
			ExpInput: &testInput{},
			ExpErr:   errors.New("early error"),
		},
		{
			Name:     "Form POST with query should overwrite partially",
			Wares:    []middleware.Transware{queryParse},
			Others:   []middleware.DecoderFactory{middleware.NewFormDecoding(schema.NewDecoder())},
			Method:   http.MethodPost,
			Path:     "?name=foo&form-image=overwritten",
			Headers:  http.Header{"Content-Type": []string{"application/x-www-form-urlencoded"}},
			Body:     `form-image=bar`,
			ExpInput: &testInput{Name: "foo", Image: "overwritten"},
		},
		{
			Name:     "Validation middleware should work on json",
			Method:   http.MethodPost,
			Wares:    []middleware.Transware{validateIngress},
			Headers:  http.Header{"Content-Type": []string{"application/json"}},
			Body:     `{"json-image": "Ïd"}`,
			ExpErr:   errors.New("Key: 'testInput.Image' Error:Field validation for 'Image' failed on the 'ascii' tag"),
			ExpInput: &testInput{Name: "", Image: "Ïd"},
		},
	} {
		t.Run(c.Name, func(t *testing.T) {
			var b io.Reader
			if c.Body != "" {
				b = bytes.NewBufferString(c.Body)
			}

			r, err := http.NewRequest(c.Method, c.Path, b)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			r.Header = c.Headers
			in := &testInput{}
			j := &middleware.JSON{}
			egress := middleware.NewEgress(j)
			ingress := middleware.NewIngress(egress, j, c.Others...)
			ingress.Use(c.Wares...)

			err = ingress.Parse(r, in)
			if fmt.Sprint(c.ExpErr) != fmt.Sprint(err) {
				t.Fatalf("Expected error: %#v, got: %#v", fmt.Sprint(c.ExpErr), fmt.Sprint(err))
			}

			if !reflect.DeepEqual(in, c.ExpInput) {
				t.Fatalf("Expected input: %#v, got: %#v", c.ExpInput, in)
			}
		})
	}
}

func TestParseIntoNil(t *testing.T) {
	h := handling.NewH(
		encoding.NewStack(encoding.NewFormEncoding(schema.NewEncoder(), schema.NewDecoder())),
	)

	r, _ := http.NewRequest("GET", "", nil)

	err := h.Parse(r, nil)
	if err != nil {
		t.Fatal("parsing into nil should be no-op")
	}
}
