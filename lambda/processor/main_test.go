package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHandler_ValidInput(t *testing.T) {
	tests := []struct {
		name  string
		event Event
	}{
		{
			name:  "valid mp3",
			event: Event{AudioID: "uploads/song.mp3", Bucket: "my-bucket", ObjectKey: "uploads/song.mp3"},
		},
		{
			name:  "valid wav",
			event: Event{AudioID: "audio.wav", Bucket: "bucket", ObjectKey: "audio.wav"},
		},
		{
			name:  "valid m4a",
			event: Event{AudioID: "track.m4a", Bucket: "bucket", ObjectKey: "track.m4a"},
		},
		{
			name:  "valid ogg",
			event: Event{AudioID: "music.ogg", Bucket: "bucket", ObjectKey: "music.ogg"},
		},
		{
			name:  "valid flac",
			event: Event{AudioID: "lossless.flac", Bucket: "bucket", ObjectKey: "lossless.flac"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := handler(context.Background(), tc.event)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if resp.Status != "PROCESSED" {
				t.Errorf("expected status PROCESSED, got %s", resp.Status)
			}
			if resp.AudioID != tc.event.AudioID {
				t.Errorf("expected audioId %s, got %s", tc.event.AudioID, resp.AudioID)
			}
		})
	}
}

func TestHandler_EmptyFields(t *testing.T) {
	tests := []struct {
		name        string
		event       Event
		errContains string
	}{
		{
			name:        "empty audioId",
			event:       Event{AudioID: "", Bucket: "bucket", ObjectKey: "file.mp3"},
			errContains: "audioId is required",
		},
		{
			name:        "empty bucket",
			event:       Event{AudioID: "file.mp3", Bucket: "", ObjectKey: "file.mp3"},
			errContains: "bucket is required",
		},
		{
			name:        "empty objectKey",
			event:       Event{AudioID: "file.mp3", Bucket: "bucket", ObjectKey: ""},
			errContains: "objectKey is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handler(context.Background(), tc.event)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("expected error to contain %q, got: %v", tc.errContains, err)
			}
		})
	}
}

func TestHandler_InvalidExtension(t *testing.T) {
	tests := []struct {
		name  string
		event Event
	}{
		{
			name:  "txt file",
			event: Event{AudioID: "notes.txt", Bucket: "bucket", ObjectKey: "notes.txt"},
		},
		{
			name:  "exe file",
			event: Event{AudioID: "app.exe", Bucket: "bucket", ObjectKey: "app.exe"},
		},
		{
			name:  "no extension",
			event: Event{AudioID: "noext", Bucket: "bucket", ObjectKey: "noext"},
		},
		{
			name:  "pdf file",
			event: Event{AudioID: "doc.pdf", Bucket: "bucket", ObjectKey: "doc.pdf"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handler(context.Background(), tc.event)
			if err == nil {
				t.Fatal("expected an error for invalid extension, got nil")
			}
			if !strings.Contains(err.Error(), "unsupported file extension") {
				t.Errorf("expected 'unsupported file extension' error, got: %v", err)
			}
		})
	}
}

func TestHandler_StructuredLogging(t *testing.T) {
	// Capture stderr (where log output goes) and stdout
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	rErr, wErr, _ := os.Pipe()
	rOut, wOut, _ := os.Pipe()
	os.Stderr = wErr
	os.Stdout = wOut

	event := Event{AudioID: "test-audio.mp3", Bucket: "my-bucket", ObjectKey: "test-audio.mp3"}
	_, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Restore
	wErr.Close()
	wOut.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, rErr)
	var stdoutBuf bytes.Buffer
	io.Copy(&stdoutBuf, rOut)

	// Combine all output
	allOutput := stderrBuf.String() + stdoutBuf.String()

	// Find JSON log lines and verify structured fields
	lines := strings.Split(allOutput, "\n")
	foundStructuredLog := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
			continue
		}
		// Check for structured log fields
		if _, hasLevel := logEntry["level"]; hasLevel {
			if _, hasMsg := logEntry["msg"]; hasMsg {
				if _, hasAudioId := logEntry["audioId"]; hasAudioId {
					foundStructuredLog = true
					break
				}
			}
		}
	}

	if !foundStructuredLog {
		t.Errorf("expected structured JSON log line with fields 'level', 'msg', 'audioId' but got output:\n%s", allOutput)
	}
}
