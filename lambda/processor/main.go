package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

// Event represents the input from the Step Functions state machine.
type Event struct {
	AudioID   string `json:"audioId"`
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"objectKey"`
}

// Response represents the output returned to the state machine.
type Response struct {
	Status  string `json:"status"`
	AudioID string `json:"audioId"`
}

// logEntry represents a structured JSON log line.
type logEntry struct {
	Level     string `json:"level"`
	Msg       string `json:"msg"`
	RequestID string `json:"requestId,omitempty"`
	AudioID   string `json:"audioId,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	ObjectKey string `json:"objectKey,omitempty"`
	Timestamp string `json:"timestamp"`
}

// structuredLog emits a JSON log line to stdout.
func structuredLog(level, msg, requestID, audioID, bucket, objectKey string) {
	entry := logEntry{
		Level:     level,
		Msg:       msg,
		RequestID: requestID,
		AudioID:   audioID,
		Bucket:    bucket,
		ObjectKey: objectKey,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(entry)
	fmt.Fprintln(os.Stdout, string(data))
}

// validExtensions lists the supported audio file formats.
var validExtensions = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".m4a":  true,
	".ogg":  true,
	".flac": true,
}

func handler(ctx context.Context, event Event) (Response, error) {
	// Extract request ID from Lambda context
	var requestID string
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		requestID = lc.AwsRequestID
	}

	_ = os.Getenv("TABLE_NAME")

	structuredLog("info", "Processing audio", requestID, event.AudioID, event.Bucket, event.ObjectKey)

	// Validate required fields
	if event.AudioID == "" {
		structuredLog("error", "validation error: audioId is required", requestID, event.AudioID, event.Bucket, event.ObjectKey)
		return Response{}, fmt.Errorf("validation error: audioId is required")
	}
	if event.Bucket == "" {
		structuredLog("error", "validation error: bucket is required", requestID, event.AudioID, event.Bucket, event.ObjectKey)
		return Response{}, fmt.Errorf("validation error: bucket is required")
	}
	if event.ObjectKey == "" {
		structuredLog("error", "validation error: objectKey is required", requestID, event.AudioID, event.Bucket, event.ObjectKey)
		return Response{}, fmt.Errorf("validation error: objectKey is required")
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(event.ObjectKey))
	if !validExtensions[ext] {
		structuredLog("error", "unsupported file extension", requestID, event.AudioID, event.Bucket, event.ObjectKey)
		return Response{}, fmt.Errorf("validation error: unsupported file extension %q, must be one of .mp3, .wav, .m4a, .ogg, .flac", ext)
	}

	structuredLog("info", "Audio processed successfully", requestID, event.AudioID, event.Bucket, event.ObjectKey)

	return Response{
		Status:  "PROCESSED",
		AudioID: event.AudioID,
	}, nil
}

func main() {
	lambda.Start(handler)
}
