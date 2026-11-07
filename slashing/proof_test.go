package slashing

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type sampleNested struct {
	Value int64 `json:"value"`
}

type sampleProof struct {
	Name   string            `json:"name"`
	Height uint64            `json:"height"`
	Hash   [4]byte           `json:"hash"`
	Blob   []byte            `json:"blob"`
	Nested *sampleNested     `json:"nested,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

func sample() sampleProof {
	return sampleProof{
		Name:   "alice<&>",
		Height: 7,
		Hash:   [4]byte{1, 2, 3, 4},
		Blob:   []byte{'A'},
		Nested: &sampleNested{Value: 5},
		Tags:   map[string]string{"a": "1", "b": "2"},
	}
}

// canonicalSample is how json.Marshal spells sample(): it escapes the HTML characters of the name.
const canonicalSample = `{"name":"alice\u003c\u0026\u003e","height":7,"hash":[1,2,3,4],"blob":"QQ==","nested":{"value":5},"tags":{"a":"1","b":"2"}}`

func TestDecodeProofAcceptsOnlyTheCanonicalSpelling(t *testing.T) {
	encoded, err := json.Marshal(sample())
	if err != nil || string(encoded) != canonicalSample {
		t.Fatalf("the fixture is not spelled as the test expects: %s (%v)", encoded, err)
	}
	decoded, err := DecodeProof[sampleProof](encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, sample()) {
		t.Fatalf("expected %+v, got %+v", sample(), decoded)
	}
	// What a proof decodes to is spelled the same way when it is encoded again, so a proof one node
	// accepted is accepted by every node.
	again, err := json.Marshal(decoded)
	if err != nil || string(again) != canonicalSample {
		t.Fatalf("expected a stable encoding, got %s (%v)", again, err)
	}
}

// Every spelling below is read by encoding/json as the very value of the canonical one. Each would
// be a transaction of its own that proves the same offence.
func TestDecodeProofRejectsOtherSpellingsOfOneValue(t *testing.T) {
	replace := func(old string, replacement string) string {
		if !strings.Contains(canonicalSample, old) {
			t.Fatalf("the fixture does not contain %q", old)
		}
		return strings.Replace(canonicalSample, old, replacement, 1)
	}
	cases := map[string]string{
		"leading whitespace":                  " " + canonicalSample,
		"a trailing space":                    canonicalSample + " ",
		"a trailing newline":                  canonicalSample + "\n",
		"padding up to a large size":          canonicalSample + strings.Repeat(" ", 4096),
		"whitespace between tokens":           replace(`"height":7`, `"height": 7`),
		"whitespace around the colon":         replace(`"height":7`, "\"height\"\t:7"),
		"fields in another order":             `{"height":7,"name":"alice\u003c\u0026\u003e","hash":[1,2,3,4],"blob":"QQ==","nested":{"value":5},"tags":{"a":"1","b":"2"}}`,
		"map keys in another order":           replace(`{"a":"1","b":"2"}`, `{"b":"2","a":"1"}`),
		"a field name in upper case":          replace(`"name"`, `"NAME"`),
		"a field name in mixed case":          replace(`"height"`, `"Height"`),
		"a character written as an escape":    replace("alice", `\u0061lice`),
		"an HTML character left unescaped":    replace(`\u003c`, "<"),
		"a line break inside base64":          replace(`"QQ=="`, `"Q\r\nQ=="`),
		"base64 with bits set past the data":  replace(`"QQ=="`, `"QR=="`),
		"a fixed-size array that is too long": replace(`[1,2,3,4]`, `[1,2,3,4,0,0]`),
		"a repeated field":                    replace(`"height":7`, `"height":6,"height":7`),
	}
	for name, proof := range cases {
		t.Run(name, func(t *testing.T) {
			var lenient sampleProof
			if err := json.Unmarshal([]byte(proof), &lenient); err != nil || !reflect.DeepEqual(lenient, sample()) {
				t.Fatalf("expected encoding/json to read the same value, got %+v (%v)", lenient, err)
			}
			decoded, err := DecodeProof[sampleProof]([]byte(proof))
			if !errors.Is(err, ErrNonCanonicalProof) && !errors.Is(err, ErrMalformedProof) {
				t.Fatalf("expected the spelling to be rejected, got %+v (%v)", decoded, err)
			}
			if !reflect.DeepEqual(decoded, sampleProof{}) {
				t.Fatalf("expected no value with the rejection, got %+v", decoded)
			}
		})
	}
}

func TestDecodeProofNamesWhatIsWrongWithAProof(t *testing.T) {
	cases := []struct {
		name  string
		proof string
		want  error
	}{
		{"not JSON", `proof`, ErrMalformedProof},
		{"empty", ``, ErrMalformedProof},
		{"truncated", canonicalSample[:len(canonicalSample)-1], ErrMalformedProof},
		{"an unknown field", strings.TrimSuffix(canonicalSample, `}`) + `,"extra":true}`, ErrMalformedProof},
		{"an unknown field in another case", strings.TrimSuffix(canonicalSample, `}`) + `,"EXTRA":true}`, ErrMalformedProof},
		{"a value of the wrong type", strings.Replace(canonicalSample, `"height":7`, `"height":"7"`, 1), ErrMalformedProof},
		{"a fraction for an integer", strings.Replace(canonicalSample, `"height":7`, `"height":7.0`, 1), ErrMalformedProof},
		{"a second document", canonicalSample + canonicalSample, ErrMalformedProof},
		{"a second value behind a space", canonicalSample + ` {}`, ErrMalformedProof},
		{"garbage behind the document", canonicalSample + `x`, ErrMalformedProof},
		{"an array", `[]`, ErrMalformedProof},
		{"null", `null`, ErrNonCanonicalProof},
		{"a field name in another case", strings.Replace(canonicalSample, `"name"`, `"NAME"`, 1), ErrNonCanonicalProof},
		{"whitespace", canonicalSample + "\n", ErrNonCanonicalProof},
	}
	for _, testCase := range cases {
		if _, err := DecodeProof[sampleProof]([]byte(testCase.proof)); !errors.Is(err, testCase.want) {
			t.Fatalf("%s: expected %v, got %v", testCase.name, testCase.want, err)
		}
	}
	// A negative zero is the integer zero to encoding/json, and not what it writes for it.
	if _, err := DecodeProof[sampleNested]([]byte(`{"value":-0}`)); !errors.Is(err, ErrNonCanonicalProof) {
		t.Fatalf("expected -0 to be rejected as non-canonical, got %v", err)
	}
	if decoded, err := DecodeProof[sampleNested]([]byte(`{"value":0}`)); err != nil || decoded.Value != 0 {
		t.Fatalf("expected 0 to decode, got %+v (%v)", decoded, err)
	}
}

// A nil slice is spelled null and an empty one "", and each decodes back to itself: both spellings
// are canonical for their value.
func TestDecodeProofKeepsEmptyAndAbsentValuesApart(t *testing.T) {
	for name, value := range map[string]sampleProof{
		"zero":  {},
		"empty": {Blob: []byte{}},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeProof[sampleProof](encoded)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(decoded, value) {
			t.Fatalf("%s: expected %+v, got %+v", name, value, decoded)
		}
	}
	if _, err := DecodeProof[sampleProof]([]byte(`{"name":"","height":0,"hash":[0,0,0,0],"blob":""}`)); err != nil {
		t.Fatalf("expected an empty blob to be canonical, got %v", err)
	}
	if _, err := DecodeProof[sampleProof]([]byte(`{"name":"","height":0,"hash":[0,0,0,0],"blob":null}`)); err != nil {
		t.Fatalf("expected a null blob to be canonical, got %v", err)
	}
}
