package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

const maxImageBytes int64 = 5_000_000
const maxRequestBytes int64 = maxImageBytes + 64*1024

type imageResult struct {
	Format    string `json:"format"`
	MimeType  string `json:"mime_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	SizeBytes int64  `json:"size_bytes"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorResponse{Error: errorDetail{Code: code, Message: message}})
}

func NewHandler(processorURL string, client *http.Client) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/v1/images/validate", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
		defer r.Body.Close()
		reader, err := r.MultipartReader()
		if err != nil {
			WriteError(w, 400, "invalid_request", "Send one image using multipart form data.")
			return
		}
		part, err := reader.NextPart()
		if err != nil || part.FormName() != "image" || part.FileName() == "" {
			writeUploadError(w, err, "Choose one image to upload.")
			return
		}
		data, err := io.ReadAll(io.LimitReader(part, maxImageBytes+1))
		if err != nil {
			writeUploadError(w, err, "The upload could not be read. Please try again.")
			return
		}
		if int64(len(data)) > maxImageBytes {
			WriteError(w, 413, "file_too_large", "Choose an image no larger than 5 MB (5,000,000 bytes).")
			return
		}
		if len(data) == 0 {
			WriteError(w, 400, "empty_file", "The selected file is empty. Choose another image.")
			return
		}
		if _, err = reader.NextPart(); err != io.EOF {
			writeUploadError(w, err, "Upload exactly one image, without additional form fields.")
			return
		}

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		// The processor identifies content; visitor filenames and MIME claims are not needed.
		filePart, err := writer.CreateFormFile("image", "upload")
		if err != nil {
			processorUnavailable(w)
			return
		}
		_, _ = filePart.Write(data)
		_ = writer.Close()
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(processorURL, "/")+"/internal/v1/images/validate", &body)
		if err != nil {
			processorUnavailable(w)
			return
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if Cloud() {
			req.Header.Set("Authorization", "Bearer "+os.Getenv("SYNTELLE_PROCESSOR_TOKEN"))
		}
		res, err := client.Do(req)
		if err != nil {
			processorUnavailable(w)
			return
		}
		defer res.Body.Close()
		payload, err := io.ReadAll(io.LimitReader(res.Body, 64*1024+1))
		if err != nil || len(payload) > 64*1024 {
			processorUnavailable(w)
			return
		}
		if res.StatusCode == http.StatusOK {
			var result imageResult
			expectedMIME := map[string]string{"JPEG": "image/jpeg", "PNG": "image/png", "WEBP": "image/webp"}
			if json.Unmarshal(payload, &result) != nil || expectedMIME[result.Format] == "" || expectedMIME[result.Format] != result.MimeType || result.Width <= 0 || result.Height <= 0 || result.SizeBytes != int64(len(data)) {
				processorUnavailable(w)
				return
			}
			WriteJSON(w, 200, result)
			return
		}
		var upstream errorResponse
		if json.Unmarshal(payload, &upstream) == nil {
			publicErrors := map[string]struct {
				status  int
				message string
			}{
				"file_too_large":             {413, "Choose an image no larger than 5 MB (5,000,000 bytes)."},
				"unsupported_format":         {415, "Choose a JPEG, PNG, or WebP image."},
				"corrupt_image":              {422, "This image could not be read. It may be damaged. Choose another image."},
				"image_dimensions_too_large": {422, "This image has too many pixels to process safely. Choose a smaller image."},
			}
			if known, ok := publicErrors[upstream.Error.Code]; ok && res.StatusCode == known.status {
				WriteError(w, known.status, upstream.Error.Code, known.message)
				return
			}
		}
		processorUnavailable(w)
	})
	return mux
}

func writeUploadError(w http.ResponseWriter, err error, message string) {
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		WriteError(w, 413, "file_too_large", "Choose an image no larger than 5 MB (5,000,000 bytes).")
		return
	}
	WriteError(w, 400, "invalid_request", message)
}

func processorUnavailable(w http.ResponseWriter) {
	WriteError(w, 503, "processor_unavailable", "Image validation is temporarily unavailable. Please try again.")
}
