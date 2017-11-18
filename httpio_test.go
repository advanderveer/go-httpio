package httpio_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	httpio "github.com/advanderveer/go-httpio"
	"github.com/advanderveer/go-httpio/encoding"
	"github.com/gorilla/schema"
	validator "gopkg.in/go-playground/validator.v9"
)

type testInput struct {
	Name     string `json:"json-name" schema:"form-name" validate:"ascii"`
	Position string `json:"position" schema:"position"`
}

type testOutput struct {
	Result string `json:"result,omitempty"`
}

func testImpl(ctx context.Context, in *testInput) (*testOutput, error) {
	return &testOutput{}, nil
}

type val struct {
	v *validator.Validate
}

func (val *val) Validate(v interface{}) error {
	return val.v.Struct(v)
}

func TestBasicUsage(t *testing.T) {
	for _, c := range []struct {
		Name      string
		Method    string
		Path      string
		Hdr       http.Header
		Body      io.Reader
		Ctrl      *httpio.Ctrl
		Impl      func(context.Context, *testInput) (*testOutput, error)
		ExpBody   string
		ExpStatus int
		ExpHdr    http.Header
	}{
		{
			Name:      "GET an empty output struct",
			Method:    http.MethodGet,
			Path:      "",
			Body:      nil,
			Ctrl:      httpio.NewCtrl(&val{validator.New()}, &encoding.JSON{}),
			ExpBody:   `{}` + "\n",
			ExpStatus: http.StatusOK,
			ExpHdr:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Impl: func(ctx context.Context, in *testInput) (*testOutput, error) {
				return &testOutput{}, nil
			},
		},
		{
			Name:      "GET return nil and error",
			Method:    http.MethodGet,
			Path:      "",
			Body:      nil,
			Ctrl:      httpio.NewCtrl(&val{validator.New()}, &encoding.JSON{}),
			ExpBody:   `{"message":"foo"}` + "\n",
			ExpStatus: http.StatusInternalServerError,
			ExpHdr:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Impl: func(ctx context.Context, in *testInput) (*testOutput, error) {
				return nil, errors.New("foo")
			},
		},
		{
			Name:      "POST with query and json body",
			Method:    http.MethodPost,
			Path:      "?form-name=bar",
			Hdr:       http.Header{"Content-Type": []string{"application/json"}},
			Body:      strings.NewReader(`{"position": "director"}`),
			Ctrl:      httpio.NewCtrl(&val{validator.New()}, &encoding.JSON{}, encoding.NewFormEncoding(schema.NewEncoder(), schema.NewDecoder())),
			ExpBody:   `{"result":"bardirector"}` + "\n",
			ExpStatus: http.StatusOK,
			ExpHdr:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Impl: func(ctx context.Context, in *testInput) (*testOutput, error) {
				return &testOutput{Result: in.Name + in.Position}, nil
			},
		},
		{
			Name:      "POST with query and form body",
			Method:    http.MethodPost,
			Path:      "?form-name=bar",
			Hdr:       http.Header{"Content-Type": []string{"application/x-www-form-urlencoded"}},
			Body:      strings.NewReader("position=director"),
			Ctrl:      httpio.NewCtrl(&val{validator.New()}, &encoding.JSON{}, encoding.NewFormEncoding(schema.NewEncoder(), schema.NewDecoder())),
			ExpBody:   `{"result":"bardirector"}` + "\n",
			ExpStatus: http.StatusOK,
			ExpHdr:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Impl: func(ctx context.Context, in *testInput) (*testOutput, error) {
				return &testOutput{Result: in.Name + in.Position}, nil
			},
		},
		{
			Name:      "POST with query and json body that doesnt pass validation",
			Method:    http.MethodPost,
			Path:      "?form-name=bíar",
			Hdr:       http.Header{"Content-Type": []string{"application/json"}},
			Body:      strings.NewReader(`{"position": "director"}`),
			Ctrl:      httpio.NewCtrl(&val{validator.New()}, &encoding.JSON{}, encoding.NewFormEncoding(schema.NewEncoder(), schema.NewDecoder())),
			ExpBody:   `{"message":"Key: 'testInput.Name' Error:Field validation for 'Name' failed on the 'ascii' tag"}` + "\n",
			ExpStatus: http.StatusBadRequest,
			ExpHdr:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Impl: func(ctx context.Context, in *testInput) (*testOutput, error) {
				return &testOutput{Result: in.Name + in.Position}, nil
			},
		},
		{
			Name:      "POST with query and json body that is invalid",
			Method:    http.MethodPost,
			Path:      "?form-name=bíar",
			Hdr:       http.Header{"Content-Type": []string{"application/json"}},
			Body:      strings.NewReader(`{"position": "director}`),
			Ctrl:      httpio.NewCtrl(&val{validator.New()}, &encoding.JSON{}, encoding.NewFormEncoding(schema.NewEncoder(), schema.NewDecoder())),
			ExpBody:   `{"message":"unexpected EOF"}` + "\n",
			ExpStatus: http.StatusBadRequest,
			ExpHdr:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
			Impl: func(ctx context.Context, in *testInput) (*testOutput, error) {
				return &testOutput{Result: in.Name + in.Position}, nil
			},
		},
	} {
		t.Run(c.Name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r, _ := http.NewRequest(c.Method, c.Path, c.Body)
			r.Header = c.Hdr
			func(w http.ResponseWriter, r *http.Request) {
				in := &testInput{}
				if render, valid := c.Ctrl.Handle(w, r, in, &testOutput{}); valid {
					render(c.Impl(r.Context(), in))
				}
			}(w, r)

			if w.Body.String() != c.ExpBody {
				t.Fatalf("expected resp body '%s', got: %s", c.ExpBody, w.Body.String())
			}

			if w.Code != c.ExpStatus {
				t.Fatalf("expected status '%d', got: %d", c.ExpStatus, w.Code)
			}

			if !reflect.DeepEqual(w.Header(), c.ExpHdr) {
				t.Fatalf("expected resp hdr '%v', got: %v", c.ExpHdr, w.Header())
			}
		})
	}

}
