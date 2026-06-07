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
	// Skeleton: this Lambda is not yet wired to the Lambda runtime API.
	// In production deployment, replace this body with:
	//   import "github.com/aws/aws-lambda-go/lambda"
	//   lambda.Start(handler)
	// That registers the handler function with the Lambda Runtime API so
	// the provided.al2023 custom runtime can invoke it on each event.
	fmt.Println("SleepAudioProcessor Lambda handler")
}
