package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
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

// validExtensions lists the supported audio file formats.
var validExtensions = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".m4a":  true,
	".ogg":  true,
	".flac": true,
}

func handler(ctx context.Context, event Event) (Response, error) {
	tableName := os.Getenv("TABLE_NAME")
	log.Printf("Processing audio: audioId=%s bucket=%s objectKey=%s table=%s",
		event.AudioID, event.Bucket, event.ObjectKey, tableName)

	input, _ := json.Marshal(event)
	fmt.Printf("Received event: %s\n", string(input))

	// Validate required fields
	if event.AudioID == "" {
		return Response{}, fmt.Errorf("validation error: audioId is required")
	}
	if event.Bucket == "" {
		return Response{}, fmt.Errorf("validation error: bucket is required")
	}
	if event.ObjectKey == "" {
		return Response{}, fmt.Errorf("validation error: objectKey is required")
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(event.ObjectKey))
	if !validExtensions[ext] {
		return Response{}, fmt.Errorf("validation error: unsupported file extension %q, must be one of .mp3, .wav, .m4a, .ogg, .flac", ext)
	}

	return Response{
		Status:  "PROCESSED",
		AudioID: event.AudioID,
	}, nil
}

func main() {
	lambda.Start(handler)
}
