package rpc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func FuzzSubmitTxRequest(f *testing.F) {
	for _, seed := range []string{
		`{"tx":"YmFuaw=="}`,
		`{"tx":"bank:plain","encoding":"plain"}`,
		`{"tx":"not-base64"}`,
		`{"tx":"","encoding":"plain"}`,
		`{"tx":"YmFuaw==","extra":true}`,
		`not-json`,
	} {
		f.Add(seed, int64(1024))
	}
	f.Fuzz(func(t *testing.T, body string, maxRequestBytes int64) {
		if maxRequestBytes < 0 {
			maxRequestBytes = -maxRequestBytes
		}
		maxRequestBytes = maxRequestBytes%4096 + 1
		request := httptest.NewRequest(http.MethodPost, "/tx", strings.NewReader(body))
		response := httptest.NewRecorder()
		tx, err := decodeSubmitTxRequest(response, request, maxRequestBytes)
		if err != nil {
			return
		}
		if len(tx) == 0 {
			t.Fatal("decoded empty transaction")
		}
	})
}

func FuzzSubmitEvidenceRequest(f *testing.F) {
	for _, seed := range []string{
		`{"type":"double_sign","validator":"alice","height":1,"proof":"cHJvb2Y="}`,
		`{"type":"double_sign","validator":"alice","height":1,"proof":"proof","encoding":"plain"}`,
		`{"type":"double_sign","validator":"","height":1,"proof":"cHJvb2Y="}`,
		`{"type":"double_sign","validator":"alice","height":0,"proof":"cHJvb2Y="}`,
		`{"type":"double_sign","validator":"alice","height":1,"proof":"bad-base64"}`,
		`not-json`,
	} {
		f.Add(seed, int64(2048))
	}
	f.Fuzz(func(t *testing.T, body string, maxRequestBytes int64) {
		if maxRequestBytes < 0 {
			maxRequestBytes = -maxRequestBytes
		}
		maxRequestBytes = maxRequestBytes%4096 + 1
		request := httptest.NewRequest(http.MethodPost, "/evidence", strings.NewReader(body))
		response := httptest.NewRecorder()
		evidence, err := decodeSubmitEvidenceRequest(response, request, maxRequestBytes)
		if err != nil {
			return
		}
		if evidence.Type == "" || evidence.Validator == "" || evidence.Height == 0 || len(evidence.Proof) == 0 {
			t.Fatalf("decoded invalid evidence: %+v", evidence)
		}
	})
}
