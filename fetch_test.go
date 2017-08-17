package fetch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	duktape "gopkg.in/olebedev/go-duktape.v3"
)

func TestStackAroundGoFetchSync(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	ctx := duktape.New()
	goFetch := goFetchSync(http.DefaultTransport)

	Define(ctx)
	defer ctx.DestroyHeap()

	ctx.PushString(ts.URL)
	ctx.PushObject() // options
	ctx.PushObject() // headers => [ url options headers ]
	ctx.PutPropString(-2, "headers")

	ctx.PushContextDump()

	if ctx.SafeToString(-1) != "ctx: top=2, stack=[\""+ts.URL+"\",{headers:{}}]" {
		t.Fatalf("Unexpected string")
	}

	if goFetch(ctx) != 1 {
		t.Fatalf("Expected 1 from goFetch")
	}

	if ctx.GetTop() != 1 {
		t.Fatalf("Expected 1 from GetTop")
	}

	resp := response{}
	if err := json.Unmarshal([]byte(ctx.JsonEncode(-1)), &resp); err != nil {
		t.Fatal(err)
	}
}

func TestGoFetchSyncExternal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html; charset=UTF-8")
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	ctx := duktape.New()
	goFetch := goFetchSync(http.DefaultTransport)

	Define(ctx)
	defer ctx.DestroyHeap()

	ctx.PushString(ts.URL)
	ctx.PushObject()                 // options => [ url {} ]
	ctx.PushObject()                 // headers => [ url {} {} ]
	ctx.PutPropString(-2, "headers") // => [ url {headers: {}} ]

	if result := goFetch(ctx); result != 1 {
		t.Fatalf("Expected goFetch result to be 1, got %d", result)
	}

	resp := response{}

	if err := json.Unmarshal([]byte(ctx.JsonEncode(-1)), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Method != http.MethodGet {
		t.Fatalf("Expected method %s, got %s", http.MethodGet, resp.Method)
	}

	if resp.Status != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, resp.Status)
	}

	if expected := fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)); resp.StatusText != expected {
		t.Fatalf("Expected status text %s, got %s", expected, resp.StatusText)
	}

	if expected := "Hello, client\n"; resp.Body != expected {
		t.Fatalf("Expected body %s, got %s", expected, resp.Body)
	}

	if expected := "text/html; charset=UTF-8"; resp.Headers.Get("Content-Type") != expected {
		t.Fatalf("Expected Content-Type to be %s, got %s", expected, resp.Headers.Get("Content-Type"))
	}
}

func TestGoFetchInternal404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "404 page not found", http.StatusNotFound)
	}))
	defer ts.Close()

	ctx := duktape.New()
	u, _ := url.Parse(ts.URL)

	DefineWithBaseURL(ctx, u)
	defer ctx.DestroyHeap()

	if err := ctx.PevalString(`fetch.goFetchSync('/', {});`); err != nil {
		t.Fatal(err)
	}

	ctx.JsonEncode(-1)
	respString := ctx.SafeToString(-1)
	resp := response{}
	json.Unmarshal([]byte(respString), &resp)

	if resp.Method != http.MethodGet {
		t.Fatalf("Expected method %s, got %s", http.MethodGet, resp.Method)
	}

	if resp.Status != http.StatusNotFound {
		t.Fatalf("Expected status %d, got %d", http.StatusNotFound, resp.Status)
	}

	if expected := fmt.Sprintf("%d %s", http.StatusNotFound, http.StatusText(http.StatusNotFound)); resp.StatusText != expected {
		t.Fatalf("Expected status text %s, got %s", expected, resp.StatusText)
	}

	if expected := "404 page not found\n"; resp.Body != expected {
		t.Fatalf("Expected body %s, got %s", expected, resp.Body)
	}
}

func TestGoFetchPromise(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "404 page not found", http.StatusNotFound)
	}))
	defer ts.Close()

	ctx := duktape.New()
	u, _ := url.Parse(ts.URL)

	DefineWithBaseURL(ctx, u)
	defer ctx.DestroyHeap()

	ch := make(chan string)
	ctx.PushGlobalGoFunction("cbk", func(co *duktape.Context) int {
		ch <- co.SafeToString(-1)
		return 0
	})

	js := `
		fetch('/404')
			.then(function(resp){
				return resp.text();
			}).then(cbk);
		`

	if err := ctx.PevalString(js); err != nil {
		t.Fatal(err)
	}

	body := <-ch

	if expected := "404 page not found\n"; body != expected {
		t.Fatalf("Expected body %s, got %s", expected, body)
	}
}

func TestGoFetchThrowsError(t *testing.T) {
	ctx := duktape.New()
	defer ctx.DestroyHeap()

	Define(ctx)

	ch := make(chan string)
	ctx.PushGlobalGoFunction("cbk", func(co *duktape.Context) int {
		ch <- co.SafeToString(-1)
		return 0
	})

	js := `
		fetch('http://sdfsdfjsdlkgjsldg.sdfgsdg')
			.catch(function(err){
				cbk(err.message);
			});
		`

	if err := ctx.PevalString(js); err != nil {
		t.Fatal(err)
	}

	body := <-ch

	if expected := "Get http://sdfsdfjsdlkgjsldg.sdfgsdg: dial tcp: lookup sdfsdfjsdlkgjsldg.sdfgsdg: no such host"; body != expected {
		t.Fatalf("Expected body %s, got %s", expected, body)
	}
}

func TestGoFetchJson(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hello": "world",
		})
	}))
	defer ts.Close()

	ctx := duktape.New()
	u, _ := url.Parse(ts.URL)

	DefineWithBaseURL(ctx, u)
	defer ctx.DestroyHeap()

	ch := make(chan bool)
	ctx.PushGlobalGoFunction("cbk", func(co *duktape.Context) int {
		ch <- co.GetType(-1).IsObject()
		return 0
	})

	js := `
		fetch('/')
			.then(function(resp){
				return resp.json();
			}).then(cbk);
	`

	if err := ctx.PevalString(js); err != nil {
		t.Fatal(err)
	}

	if isObject := <-ch; !isObject {
		t.Fatal("Expected IsObject to be true")
	}
}

func TestGlobals(t *testing.T) {
	testCases := []struct {
		js     string
		typeOf string
	}{
		{`typeof fetch;`, "function"},
		{`typeof fetch.goFetchSync;`, "function"},
		{`typeof fetch.Promise;`, "function"},
	}
	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("#%d", idx), func(t *testing.T) {
			ctx := duktape.New()
			Define(ctx)
			defer ctx.Destroy()

			if err := ctx.PevalString(`typeof fetch;`); err != nil {
				t.Fatal(err)
			}

			got := ctx.SafeToString(-1)
			if got != tc.typeOf {
				t.Fatalf("Expected typeOf of %s, got %s", tc.typeOf, got)
			}
		})
	}
}
