//go:generate go-bindata -debug -pkg fetch -o bindata.go ./dist
package fetch

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"gopkg.in/olebedev/go-duktape.v3"
)

var bundle string

func init() {
	b, err := Asset("dist/bundle.js")
	must(err)
	bundle = string(b)
}

func Define(c *duktape.Context) {
	DefineWithRoundTripper(c, http.DefaultTransport)
}

func DefineWithBaseURL(c *duktape.Context, u *url.URL) {
	DefineWithRoundTripper(c, roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = u.Scheme
		r.URL.Host = u.Host
		return http.DefaultTransport.RoundTrip(r)
	}))
}

func DefineWithRoundTripper(c *duktape.Context, rt http.RoundTripper) {
	c.PushTimers()
	must(c.PevalString(bundle))
	c.Pop()

	c.PushGlobalObject()
	c.GetPropString(-1, "fetch")
	c.PushGoFunction(goFetchSync(rt))
	c.PutPropString(-2, "goFetchSync")
	c.Pop2()
}

func DefineWithHandler(c *duktape.Context, h http.Handler) {
	DefineWithRoundTripper(c, roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, r)

		return &http.Response{
			Request:    r,
			StatusCode: recorder.Code,
			Status:     http.StatusText(recorder.Code),
			Header:     recorder.HeaderMap,
			Body:       ioutil.NopCloser(recorder.Body),
		}, nil
	}))
}

func goFetchSync(rt http.RoundTripper) func(*duktape.Context) int {
	return func(c *duktape.Context) int {
		var opts = struct {
			URL     string      `json:"url"`
			Method  string      `json:"method"`
			Headers http.Header `json:"headers"`
			Body    string      `json:"body"`
		}{
			URL:     c.SafeToString(0),
			Method:  http.MethodGet,
			Headers: http.Header{},
		}

		err := json.Unmarshal([]byte(c.JsonEncode(1)), &opts)
		if err != nil {
			c.Pop2()
			c.PushErrorObject(1, "%v", err)
			return 1
		}

		req, err := http.NewRequest(opts.Method, opts.URL, strings.NewReader(opts.Body))
		if err != nil {
			c.Pop2()
			c.PushErrorObject(2, "%v", err)
			return 1
		}

		resp, err := doRequest(req, rt)
		if err != nil {
			c.Pop2()
			c.PushErrorObject(3, "%v", err)
			return 1
		}

		j, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			c.Pop2()
			c.PushErrorObject(4, "%v", err)
			return 1
		}

		c.Pop3()
		c.PushString(string(j))
		c.JsonDecode(-1)
		return 1
	}
}

type response struct {
	URL        string      `json:"url"`
	Method     string      `json:"method"`
	Headers    http.Header `json:"headers"`
	Body       string      `json:"body"`
	Status     int         `json:"status"`
	StatusText string      `json:"statusText,omitempty"`
}

func doRequest(req *http.Request, rt http.RoundTripper) (response, error) {
	client := http.Client{
		Transport: rt,
	}

	httpResp, err := client.Do(req)
	if err != nil {
		return response{}, err
	}

	resp := response{
		URL:        req.URL.String(),
		Method:     req.Method,
		Headers:    httpResp.Header,
		Status:     httpResp.StatusCode,
		StatusText: httpResp.Status,
	}

	defer httpResp.Body.Close()
	body, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		return resp, err
	}

	resp.Body = string(body)
	return resp, nil
}

type roundTripperFunc func(r *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
