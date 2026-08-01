package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func WriteHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

func WriteRaw(w http.ResponseWriter, event string, data []byte) error {
	var err error
	if event != "" {
		_, err = fmt.Fprintf(w, "event: %s\n", event)
	}
	if err == nil {
		_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	}
	if err == nil {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	return err
}

func WriteEvent(w http.ResponseWriter, event string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return WriteRaw(w, event, payload)
}

func WriteDone(w http.ResponseWriter) error {
	_, err := fmt.Fprintf(w, "data: [DONE]\n\n")
	if err == nil {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	return err
}

func WriteError(w http.ResponseWriter, err error) error {
	we := WriteEvent(w, "error", map[string]string{"error": err.Error()})
	if de := WriteDone(w); de != nil && we == nil {
		we = de
	}
	return we
}
