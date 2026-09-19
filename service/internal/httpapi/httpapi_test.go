package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/GhandyP/environmental-impact-gemma4/service/internal/config"
	"github.com/GhandyP/environmental-impact-gemma4/service/internal/llm"
	"github.com/GhandyP/environmental-impact-gemma4/service/web"
)

type apiFake struct {
	name, model string
	available   bool
	response    string
	err         error
}

func (f apiFake) Name() string                   { return f.name }
func (f apiFake) Model() string                  { return f.model }
func (f apiFake) Available(context.Context) bool { return f.available }
func (f apiFake) Info() llm.ProviderInfo {
	return llm.ProviderInfo{Name: f.name, Model: f.model, Available: f.available}
}
func (f apiFake) Generate(context.Context, string, []byte, string) (string, error) {
	return f.response, f.err
}

const apiJSON = `{"hazard_level":"low","summary":"safe","confidence":0}`

func pngBytes() []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	return b.Bytes()
}
func noisyPNGBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			v := uint8((x*37 + y*53 + x*y*11) % 256)
			img.SetRGBA(x, y, color.RGBA{v, v ^ 0x5a, v ^ 0xa5, 0xff})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// filePart appends one file part to a multipart writer. It centralizes the
// net/textproto MIMEHeader that multipart.Writer requires, so every test
// builds parts the same way.
func filePart(t *testing.T, mw *multipart.Writer, field, filename, contentType string, data []byte) {
	t.Helper()
	h := textproto.MIMEHeader{
		"Content-Disposition": {fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename)},
		"Content-Type":        {contentType},
	}
	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
}
func multipartRequest(t *testing.T, data []byte, lang string) *http.Request {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	filePart(t, mw, "image", "x.bin", "application/octet-stream", data)
	mw.WriteField("lang", lang)
	mw.Close()
	r := httptest.NewRequest(http.MethodPost, "/analyze", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}
func testHandler(a []llm.Analyzer, max int64) http.Handler {
	return NewHandler(config.Config{MaxImageBytes: max, ProviderTimeout: time.Second}, a)
}
func TestAnalyzeMultipart(t *testing.T) {
	r := multipartRequest(t, pngBytes(), "es")
	w := httptest.NewRecorder()
	testHandler([]llm.Analyzer{apiFake{"fake", "m", true, apiJSON, nil}}, 10000).ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("X-EIA-Provider") != "fake" {
		t.Fatalf("status %d header %q body %s", w.Code, w.Header().Get("X-EIA-Provider"), w.Body)
	}
	for _, s := range []string{`"provider"`, `"model"`, `"lang"`, `"analysis"`} {
		if !strings.Contains(w.Body.String(), s) {
			t.Error("missing " + s)
		}
	}
}
func TestAnalyzeMultipartBodyLimit(t *testing.T) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	filePart(t, mw, "image", "large.png", "image/png", bytes.Repeat([]byte{1}, 4096))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/analyze", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	testHandler([]llm.Analyzer{apiFake{"f", "m", true, apiJSON, nil}}, 1024).ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized multipart body status %d, body %s", w.Code, w.Body)
	}
}
func TestAnalyzeMultipartEnvelopeSlack(t *testing.T) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	filePart(t, mw, "image", "large.png", "image/png", bytes.Repeat([]byte{1}, 200<<10))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/analyze", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	testHandler([]llm.Analyzer{apiFake{"f", "m", true, apiJSON, nil}}, 1024).ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("multipart envelope status %d, body %s", w.Code, w.Body)
	}
}
func TestAnalyzeMultipartEnvelopeAccepted(t *testing.T) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	filePart(t, mw, "image", "a-very-long-image-filename-that-expands-the-envelope.png", "image/png", pngBytes())
	for i := 0; i < 3; i++ {
		filePart(t, mw, "extra-file",
			"another-very-long-filename-that-expands-the-envelope-"+string(rune('a'+i))+".txt",
			"text/plain", []byte("extra"))
	}
	for i := 0; i < 20; i++ {
		if err := mw.WriteField("extra-"+strings.Repeat("x", 32)+string(rune('a'+i)), strings.Repeat("v", 1024)); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/analyze", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	testHandler([]llm.Analyzer{apiFake{"f", "m", true, apiJSON, nil}}, 4096).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("multipart envelope accepted status %d, body %s", w.Code, w.Body)
	}
}
func TestAnalyzeBadInputs(t *testing.T) {
	h := testHandler([]llm.Analyzer{apiFake{"f", "m", true, apiJSON, nil}}, 100)
	for _, data := range [][]byte{{1, 2, 3}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, multipartRequest(t, data, "en"))
		if w.Code != 400 {
			t.Errorf("bad image status %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, multipartRequest(t, bytes.Repeat([]byte{1}, 101), "en"))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized image status %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, multipartRequest(t, pngBytes(), "xx"))
	if w.Code != 400 {
		t.Errorf("bad lang status %d", w.Code)
	}
}
func TestAnalyzeJSONBody(t *testing.T) {
	h := testHandler([]llm.Analyzer{apiFake{"f", "m", true, apiJSON, nil}}, 10000)
	body := `{"image_b64":"` + base64.StdEncoding.EncodeToString(pngBytes()) + `","lang":"en"}`
	r := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("small JSON status %d, body %s", w.Code, w.Body)
	}

	img := noisyPNGBytes()
	body = `{"image_b64":"` + base64.StdEncoding.EncodeToString(img) + `","lang":"en"}`
	r = httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	testHandler([]llm.Analyzer{apiFake{"f", "m", true, apiJSON, nil}}, int64(len(img))).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("noisy JSON status %d, image size %d, body %s", w.Code, len(img), w.Body)
	}

	largeB64 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 200<<10))
	body = `{"image_b64":"` + largeB64 + `","lang":"en"}`
	r = httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	testHandler([]llm.Analyzer{apiFake{"f", "m", true, apiJSON, nil}}, 1024).ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized JSON status %d, body %s", w.Code, w.Body)
	}
}
func TestAnalyzeProviderErrors(t *testing.T) {
	w := httptest.NewRecorder()
	testHandler([]llm.Analyzer{apiFake{"f", "m", true, "garbage", nil}}, 10000).ServeHTTP(w, multipartRequest(t, pngBytes(), "en"))
	if w.Code != 502 {
		t.Errorf("extract status %d", w.Code)
	}
	w = httptest.NewRecorder()
	testHandler(nil, 10000).ServeHTTP(w, multipartRequest(t, pngBytes(), "en"))
	if w.Code != 503 {
		t.Errorf("no provider status %d", w.Code)
	}
}
func TestHealthAndHome(t *testing.T) {
	h := testHandler([]llm.Analyzer{apiFake{"a", "1", true, "", nil}, apiFake{"b", "2", false, "", nil}}, 1000)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var out struct {
		Providers []llm.ProviderInfo `json:"providers"`
	}
	if json.Unmarshal(w.Body.Bytes(), &out) != nil || len(out.Providers) != 2 {
		t.Fatalf("health: %s", w.Body)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(w.Body.String(), "EIA Service") {
		t.Error("missing title")
	}
}

// TestUIReferencesOnlyDefinedIdentifiers guards against the class of bug that
// broke the analyze button in the browser: the inline script called a `show()`
// helper that was never defined, so the click handler threw before reaching
// fetch and the request was never sent. curl-based tests could not see it.
func TestUIRendersAndDefinesItsHandlers(t *testing.T) {
	html := web.IndexHTML()
	for _, want := range []string{
		`id="go"`, `id="file"`, `id="status"`, `id="toggleJson"`, `id="jsonWrap"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("UI is missing element %s", want)
		}
	}
	// Every function the handlers call must be declared in the script.
	for _, fn := range []string{"show", "pick", "renderResult", "renderLists", "renderHealth", "applyLang", "esc"} {
		if !strings.Contains(html, "function "+fn+"(") && !strings.Contains(html, "const "+fn+" =") {
			t.Errorf("UI calls %s() but never declares it", fn)
		}
	}
	// The raw JSON panel must be reachable: it starts hidden via .json and the
	// toggle needs an explicit rule that makes it visible again.
	if !strings.Contains(html, ".json.open") {
		t.Error("UI has no .json.open rule, so the raw JSON toggle can never reveal the panel")
	}
}
