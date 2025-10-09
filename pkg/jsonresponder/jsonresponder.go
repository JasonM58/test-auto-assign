package jsonresponder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ionextai/git-scrapper/pkg/errorshelper"
)

const (
	JSONContentType = "application/json"
	JSONCharset     = "utf-8"
)

// JSONResponder JSON response writer for net/http
type JSONResponder interface {
	Write(w http.ResponseWriter, status int, data interface{})
	Detail(w http.ResponseWriter, status int, data interface{})
	Error(w http.ResponseWriter, status int, error ErrorContent)
	WriteSimpleMessage(w http.ResponseWriter, status int, data interface{})
	WriteSimpleError(w http.ResponseWriter, status int, errorMsg interface{})
	NoContent(w http.ResponseWriter)
}

type Response struct {
	Metadata      Metadata      `json:"metadata"`
	Data          interface{}   `json:"data"`
	ErrorResponse ErrorResponse `json:"error"`
}

type DetailResponse struct {
	Data interface{} `json:"data"`
}

type ErrorData struct {
	Errors []errorshelper.ValidationError `json:"error_messages"`
}

type Metadata struct {
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Cursor  string `json:"cursor"`
}

// ErrorContent body content for error response
type ErrorContent interface {
	Message() []errorshelper.ValidationError
}

// ErrorResponse error message contain error id and localized message
type ErrorResponse struct {
	Message []errorshelper.ValidationError `json:"error_messages"`
}

type Error struct {
	Message interface{} `json:"message"`
}

type SimpleError struct {
	Error Error `json:"error"`
}

// JSON json response object
type jsonResponder struct {
	contentType string
}

// NewDefaultJSONResponder construct new JSON responder with default mime type and charset
func NewDefaultJSONResponder() JSONResponder {
	return &jsonResponder{
		contentType: fmt.Sprintf("%s; charset=%s", JSONContentType, JSONCharset),
	}
}

// Write write raw data to response writer
func (c *jsonResponder) Write(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", c.contentType)
	w.WriteHeader(status)
	if data == nil {
		return
	}

	content, _ := json.Marshal(data)
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	_, _ = w.Write(content)
}

// Data raw content wrapped in `data` key.
// Use this method to return data list with optional pagination and filter in the future.
func (c *jsonResponder) Detail(w http.ResponseWriter, status int, data interface{}) {
	content := DetailResponse{
		Data: data,
	}
	c.Write(w, status, content)
}

// Error write error content with id and multilang message
func (c *jsonResponder) Error(w http.ResponseWriter, status int, error ErrorContent) {
	// content := ErrorResponse{Message: error.Message()}

	// c.Write(w, status, ErrorData{Errors: []ErrorResponse{content}})
	c.Write(w, status, ErrorData{Errors: error.Message()})
}

func (c *jsonResponder) WriteSimpleMessage(w http.ResponseWriter, status int, data interface{}) {
	c.Write(w, status, data)
}

func (c *jsonResponder) WriteSimpleError(w http.ResponseWriter, status int, errorMsg interface{}) {
	data := SimpleError{
		Error: Error{Message: errorMsg},
	}
	c.Write(w, status, data)
}

func (c *jsonResponder) NoContent(w http.ResponseWriter) {
	c.Write(w, http.StatusNoContent, nil)
}
