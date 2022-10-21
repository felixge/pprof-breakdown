package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

//go:embed testdata/input.json
var inputJSON []byte

func TestMain(m *testing.M) {
	tracer.Start(
		tracer.WithEnv("test"),
		tracer.WithService("http"),
		tracer.WithServiceVersion("0.0.1"),
	)
	defer tracer.Stop()

	os.Exit(m.Run())
}

func BenchmarkHTTP(b *testing.B) {
	writeError := func(w http.ResponseWriter, err error) bool {
		if err == nil {
			return false
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%s\n", err)
		return true
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		var m interface{}
		d := json.NewDecoder(r.Body)
		if writeError(w, d.Decode(&m)) {
			return
		}
		e := json.NewEncoder(w)
		writeError(w, e.Encode(m))
	}

	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			r := httptest.NewRequest("POST", "/echo-json", bytes.NewReader(inputJSON))
			w := httptest.NewRecorder()
			httptrace.TraceAndServe(http.HandlerFunc(handler), w, r, nil)
			if w.Result().StatusCode != http.StatusOK {
				b.Fatal(w.Result().StatusCode)
			}
		}
	})
}
