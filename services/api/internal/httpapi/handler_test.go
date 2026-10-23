package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func uploadRequest(t *testing.T, data []byte, names ...string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range names {
		part, err := writer.CreateFormFile(name, "misleading.txt")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/images/validate", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestHealth(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler("", http.DefaultClient).ServeHTTP(recorder, httptest.NewRequest("GET", "/healthz", nil))
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health response: %s", recorder.Body.String())
	}
}

func TestUploadValidation(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		fields []string
		status int
		code   string
	}{
		{"accepted", []byte("image bytes"), []string{"image"}, 200, ""},
		{"exact boundary", bytes.Repeat([]byte{1}, int(maxImageBytes)), []string{"image"}, 200, ""},
		{"over boundary", bytes.Repeat([]byte{1}, int(maxImageBytes+1)), []string{"image"}, 413, "file_too_large"},
		{"empty", nil, []string{"image"}, 400, "empty_file"},
		{"missing", nil, nil, 400, "invalid_request"},
		{"wrong field", []byte{1}, []string{"photo"}, 400, "invalid_request"},
		{"multiple", []byte{1}, []string{"image", "image"}, 400, "invalid_request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var called atomic.Bool
			processor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called.Store(true)
				if r.URL.Path != "/internal/v1/images/validate" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if err := r.ParseMultipartForm(maxRequestBytes); err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				defer r.MultipartForm.RemoveAll()
				file, header, err := r.FormFile("image")
				if err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				defer file.Close()
				actual, _ := io.ReadAll(file)
				if !bytes.Equal(actual, test.data) || header.Filename != "upload" {
					t.Error("forwarded content or filename differs")
				}
				WriteJSON(w, 200, imageResult{Format: "PNG", MimeType: "image/png", Width: 3, Height: 5, SizeBytes: int64(len(actual))})
			}))
			defer processor.Close()
			recorder := httptest.NewRecorder()
			NewHandler(processor.URL, processor.Client()).ServeHTTP(recorder, uploadRequest(t, test.data, test.fields...))
			if recorder.Code != test.status {
				t.Fatalf("got %d, want %d: %s", recorder.Code, test.status, recorder.Body.String())
			}
			if called.Load() != (test.status == 200) {
				t.Fatalf("unexpected processor call: %t", called.Load())
			}
			if test.code != "" {
				var response errorResponse
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Error.Code != test.code {
					t.Fatalf("unexpected error: %s", recorder.Body.String())
				}
			}
		})
	}
}

func TestProcessorResponses(t *testing.T) {
	tests := []struct {
		name           string
		upstreamStatus int
		payload        string
		expectedStatus int
		expectedCode   string
	}{
		{"unsupported", 415, `{"error":{"code":"unsupported_format","message":"internal secret"}}`, 415, "unsupported_format"},
		{"corrupt", 422, `{"error":{"code":"corrupt_image","message":"internal secret"}}`, 422, "corrupt_image"},
		{"oversized", 413, `{"error":{"code":"file_too_large","message":"internal secret"}}`, 413, "file_too_large"},
		{"pixel protection", 422, `{"error":{"code":"image_dimensions_too_large"}}`, 422, "image_dimensions_too_large"},
		{"internal failure", 500, "internal secret", 503, "processor_unavailable"},
		{"bad json", 200, "internal secret", 503, "processor_unavailable"},
		{"invalid dimensions", 200, `{"format":"PNG","mime_type":"image/png","width":0,"height":2,"size_bytes":1}`, 503, "processor_unavailable"},
		{"mismatched size", 200, `{"format":"PNG","mime_type":"image/png","width":2,"height":2,"size_bytes":5}`, 503, "processor_unavailable"},
		{"wrong MIME", 200, `{"format":"PNG","mime_type":"text/plain","width":2,"height":2,"size_bytes":1}`, 503, "processor_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			processor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.upstreamStatus)
				_, _ = io.WriteString(w, test.payload)
			}))
			defer processor.Close()
			recorder := httptest.NewRecorder()
			NewHandler(processor.URL, processor.Client()).ServeHTTP(recorder, uploadRequest(t, []byte{1}, "image"))
			if recorder.Code != test.expectedStatus || !strings.Contains(recorder.Body.String(), test.expectedCode) || strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestProcessorUnavailable(t *testing.T) {
	processor := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	processor.Close()
	recorder := httptest.NewRecorder()
	NewHandler(processor.URL, &http.Client{Timeout: time.Second}).ServeHTTP(recorder, uploadRequest(t, []byte{1}, "image"))
	if recorder.Code != 503 || !strings.Contains(recorder.Body.String(), "processor_unavailable") {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestProcessorTimeout(t *testing.T) {
	processor := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { time.Sleep(100 * time.Millisecond) }))
	defer processor.Close()
	recorder := httptest.NewRecorder()
	NewHandler(processor.URL, &http.Client{Timeout: 20 * time.Millisecond}).ServeHTTP(recorder, uploadRequest(t, []byte{1}, "image"))
	if recorder.Code != 503 {
		t.Fatalf("got %d, want 503", recorder.Code)
	}
}

func TestMalformedMultipart(t *testing.T) {
	for _, contentType := range []string{"application/json", "multipart/form-data; boundary=missing"} {
		req := httptest.NewRequest("POST", "/api/v1/images/validate", strings.NewReader("not multipart data"))
		req.Header.Set("Content-Type", contentType)
		recorder := httptest.NewRecorder()
		NewHandler("", http.DefaultClient).ServeHTTP(recorder, req)
		if recorder.Code != 400 {
			t.Fatalf("got %d for %s", recorder.Code, contentType)
		}
	}
}
