package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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

func handler(ctx context.Context, event Event) (Response, error) {
	tableName := os.Getenv("TABLE_NAME")
	log.Printf("Processing audio: audioId=%s bucket=%s objectKey=%s table=%s",
		event.AudioID, event.Bucket, event.ObjectKey, tableName)

	input, _ := json.Marshal(event)
	fmt.Printf("Received event: %s\n", string(input))

	return Response{
		Status:  "PROCESSED",
		AudioID: event.AudioID,
	}, nil
}

func main() {
	// Placeholder: in a real deployment this would use
	// github.com/aws/aws-lambda-go/lambda to start the handler.
	// For now, this file serves as the Lambda asset source.
	fmt.Println("SleepAudioProcessor Lambda handler")
}
