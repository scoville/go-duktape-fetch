package fetch

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
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
		must(err)

		req, err := http.NewRequest(opts.Method, opts.URL, strings.NewReader(opts.Body))
		must(err)

		resp := doRequest(req, rt)

		// if strings.HasPrefix(url, "http") || strings.HasPrefix(url, "//") {
		// 	resp = fetchHttp(url, opts)
		// } else if strings.HasPrefix(url, "/") {
		// 	resp = fetchHandlerFunc(server, url, opts)
		// } else {
		// 	return duktape.ErrRetURI
		// }

		j, err := json.MarshalIndent(resp, "", "  ")
		must(err)

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
	Errors     []error     `json:"errors"`
}

func doRequest(req *http.Request, rt http.RoundTripper) response {
	client := http.Client{
		Transport: rt,
	}

	httpResp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	resp := response{
		URL:        req.URL.String(),
		Method:     req.Method,
		Headers:    httpResp.Header,
		Status:     httpResp.StatusCode,
		StatusText: httpResp.Status,
		Errors:     []error{},
	}

	defer httpResp.Body.Close()
	body, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		resp.Errors = append(resp.Errors, err)
		return resp
	}

	resp.Body = string(body)
	return resp

	// var body string
	// var errs []error

	// client := gorequest.New()
	// switch opts.Method {
	// case gorequest.HEAD:
	// 	resp, body, errs = client.Head(url).End()
	// case gorequest.GET:
	// 	resp, body, errs = client.Get(url).End()
	// case gorequest.POST:
	// 	resp, body, errs = client.Post(url).Query(opts.Body).End()
	// case gorequest.PUT:
	// 	resp, body, errs = client.Put(url).Query(opts.Body).End()
	// case gorequest.PATCH:
	// 	resp, body, errs = client.Patch(url).Query(opts.Body).End()
	// case gorequest.DELETE:
	// 	resp, body, errs = client.Delete(url).End()
	// }

	// result := response{
	// 	options:    opts,
	// 	Status:     resp.StatusCode,
	// 	StatusText: resp.Status,
	// 	Errors:     errs,
	// }
	// result.Body = body
	// result.Headers = resp.Header
	// return result
}

// func fetchHandlerFunc(server http.Handler, url string, opts options) response {
// 	result := response{
// 		options: opts,
// 		Errors:  []error{},
// 	}

// 	if server == nil {
// 		result.Errors = append(result.Errors, errors.New("`http.Handler` isn't set yet"))
// 		result.Status = http.StatusInternalServerError
// 	}

// 	b := bytes.NewBufferString(opts.Body)
// 	res := httptest.NewRecorder()
// 	req, err := http.NewRequest(opts.Method, url, b)

// 	if err != nil {
// 		result.Errors = []error{err}
// 		return result
// 	}

// 	req.Header = opts.Headers
// 	server.ServeHTTP(res, req)
// 	result.Status = res.Code
// 	result.Headers = res.Header()
// 	result.Body = res.Body.String()
// 	return result
// }

type roundTripperFunc func(r *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
