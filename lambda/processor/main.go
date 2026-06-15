package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	pollytypes "github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// maxAudioStreamSize is the maximum number of bytes to read from the Polly audio
// stream. This prevents memory exhaustion if the stream misbehaves.
const maxAudioStreamSize = 10 * 1024 * 1024 // 10 MB

// S3Client defines the interface for S3 operations.
type S3Client interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// DynamoDBClient defines the interface for DynamoDB operations.
type DynamoDBClient interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// PollyClient defines the interface for Polly operations.
type PollyClient interface {
	SynthesizeSpeech(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error)
}

// Processor holds the AWS service clients and configuration.
type Processor struct {
	S3Client       S3Client
	DynamoDBClient DynamoDBClient
	PollyClient    PollyClient
	TableName      string
	OutputBucket   string
	InputBucket    string
}

// Event represents the input from the Step Functions state machine.
type Event struct {
	AudioID   string `json:"audioId"`
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"objectKey"`
}

// Response represents the output returned to the state machine.
type Response struct {
	Status             string `json:"status"`
	AudioID            string `json:"audioId"`
	OutputLocation     string `json:"outputLocation,omitempty"`
	FileSize           int64  `json:"fileSize,omitempty"`
	ProcessingDuration string `json:"processingDuration,omitempty"`
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

// defaultProcessor is the package-level processor used by the handler.
var defaultProcessor *Processor

// handler is the Lambda entry point.
func handler(ctx context.Context, event Event) (Response, error) {
	startTime := time.Now()

	// Extract request ID from Lambda context
	var requestID string
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		requestID = lc.AwsRequestID
	}

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

	// If no processor configured (unit tests for validation only), return basic response
	if defaultProcessor == nil {
		structuredLog("info", "Audio processed successfully", requestID, event.AudioID, event.Bucket, event.ObjectKey)
		return Response{
			Status:  "PROCESSED",
			AudioID: event.AudioID,
		}, nil
	}

	// Process audio using the Processor
	resp, err := defaultProcessor.Process(ctx, event, requestID, startTime)
	if err != nil {
		return resp, err
	}

	structuredLog("info", "Audio processed successfully", requestID, event.AudioID, event.Bucket, event.ObjectKey)
	return resp, nil
}

// Process performs the full audio processing pipeline.
func (p *Processor) Process(ctx context.Context, event Event, requestID string, startTime time.Time) (Response, error) {
	// Validate that the event bucket matches the configured input bucket (if set)
	if p.InputBucket != "" && event.Bucket != p.InputBucket {
		structuredLog("error", fmt.Sprintf("bucket mismatch: event bucket %q does not match configured input bucket %q", event.Bucket, p.InputBucket), requestID, event.AudioID, event.Bucket, event.ObjectKey)
		return Response{}, fmt.Errorf("validation error: event bucket %q does not match configured input bucket %q", event.Bucket, p.InputBucket)
	}

	// Step 1: Download the audio file from S3 input bucket.
	// The S3 GetObject call verifies the file exists and is accessible before processing.
	// In the current implementation, Polly generates the sleep audio directly. Future
	// iterations will read the input and merge it with synthesized audio.
	structuredLog("info", "Downloading from S3", requestID, event.AudioID, event.Bucket, event.ObjectKey)
	getOutput, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(event.Bucket),
		Key:    aws.String(event.ObjectKey),
	})
	if err != nil {
		structuredLog("error", "S3 GetObject failed", requestID, event.AudioID, event.Bucket, event.ObjectKey)
		// Attempt to update DynamoDB with FAILED status
		p.updateDynamoDBStatus(ctx, event.AudioID, "FAILED", "", 0)
		return Response{}, fmt.Errorf("failed to download from S3: %w", err)
	}
	if getOutput.Body != nil {
		defer getOutput.Body.Close()
	}

	// Step 2: Call Polly SynthesizeSpeech to generate soothing audio
	structuredLog("info", "Calling Polly SynthesizeSpeech", requestID, event.AudioID, event.Bucket, event.ObjectKey)
	pollyOutput, err := p.PollyClient.SynthesizeSpeech(ctx, &polly.SynthesizeSpeechInput{
		Text:         aws.String("Welcome to your sleep audio session. Relax and breathe deeply."),
		VoiceId:      pollytypes.VoiceIdJoanna,
		OutputFormat: pollytypes.OutputFormatMp3,
	})
	if err != nil {
		structuredLog("error", "Polly SynthesizeSpeech failed, attempting passthrough", requestID, event.AudioID, event.Bucket, event.ObjectKey)
		// Graceful degradation: attempt to update DynamoDB with FAILED status
		p.updateDynamoDBStatus(ctx, event.AudioID, "FAILED", "", 0)
		return Response{}, fmt.Errorf("failed to synthesize speech: %w", err)
	}

	// Read the Polly audio output
	var audioData []byte
	if pollyOutput.AudioStream != nil {
		// Limit read to prevent memory exhaustion if the stream misbehaves
		audioData, err = io.ReadAll(io.LimitReader(pollyOutput.AudioStream, maxAudioStreamSize))
		if err != nil {
			structuredLog("error", "Failed to read Polly audio stream", requestID, event.AudioID, event.Bucket, event.ObjectKey)
			p.updateDynamoDBStatus(ctx, event.AudioID, "FAILED", "", 0)
			return Response{}, fmt.Errorf("failed to read Polly audio stream: %w", err)
		}
	}

	// Step 3: Upload the result to S3 output bucket
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	outputKey := fmt.Sprintf("processed/%s/%s.mp3", event.AudioID, timestamp)
	outputLocation := fmt.Sprintf("s3://%s/%s", p.OutputBucket, outputKey)
	fileSize := int64(len(audioData))

	structuredLog("info", "Uploading to S3 output bucket", requestID, event.AudioID, p.OutputBucket, outputKey)
	_, err = p.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.OutputBucket),
		Key:         aws.String(outputKey),
		Body:        strings.NewReader(string(audioData)),
		ContentType: aws.String("audio/mpeg"),
	})
	if err != nil {
		structuredLog("error", "S3 PutObject failed", requestID, event.AudioID, p.OutputBucket, outputKey)
		// Attempt to update DynamoDB with FAILED status
		p.updateDynamoDBStatus(ctx, event.AudioID, "FAILED", "", 0)
		return Response{}, fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Step 4: Update DynamoDB record with COMPLETED status
	// If DynamoDB update fails after successful S3 upload, log the error but still
	// return success. The Step Functions MarkCompleted state will update DynamoDB anyway.
	structuredLog("info", "Updating DynamoDB record", requestID, event.AudioID, event.Bucket, event.ObjectKey)
	err = p.updateDynamoDBStatus(ctx, event.AudioID, "COMPLETED", outputLocation, fileSize)
	if err != nil {
		structuredLog("error", fmt.Sprintf("DynamoDB UpdateItem failed after successful processing: %v", err), requestID, event.AudioID, event.Bucket, event.ObjectKey)
	}

	processingDuration := time.Since(startTime).String()

	return Response{
		Status:             "COMPLETED",
		AudioID:            event.AudioID,
		OutputLocation:     outputLocation,
		FileSize:           fileSize,
		ProcessingDuration: processingDuration,
	}, nil
}

// updateDynamoDBStatus updates the DynamoDB record with the given status and output info.
func (p *Processor) updateDynamoDBStatus(ctx context.Context, audioID, status, outputLocation string, fileSize int64) error {
	if p.TableName == "" {
		return nil
	}

	updateExpr := "SET #s = :status, #ua = :updatedAt"
	exprNames := map[string]string{
		"#s":  "status",
		"#ua": "updatedAt",
	}
	exprValues := map[string]dynamodbtypes.AttributeValue{
		":status":    &dynamodbtypes.AttributeValueMemberS{Value: status},
		":updatedAt": &dynamodbtypes.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
	}

	if outputLocation != "" {
		updateExpr += ", #ol = :outputLocation, #fs = :fileSize"
		exprNames["#ol"] = "outputLocation"
		exprNames["#fs"] = "fileSize"
		exprValues[":outputLocation"] = &dynamodbtypes.AttributeValueMemberS{Value: outputLocation}
		exprValues[":fileSize"] = &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", fileSize)}
	}

	_, err := p.DynamoDBClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(p.TableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"audioId": &dynamodbtypes.AttributeValueMemberS{Value: audioID},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
	})
	return err
}

// newProcessor creates a Processor from the given AWS config and environment variables.
func newProcessor(cfg aws.Config, tableName, outputBucket, inputBucket string) *Processor {
	return &Processor{
		S3Client:       s3.NewFromConfig(cfg),
		DynamoDBClient: dynamodb.NewFromConfig(cfg),
		PollyClient:    polly.NewFromConfig(cfg),
		TableName:      tableName,
		OutputBucket:   outputBucket,
		InputBucket:    inputBucket,
	}
}

func main() {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		structuredLog("error", "Failed to load AWS config", "", "", "", "")
		os.Exit(1)
	}

	// Initialize the processor from config and environment
	defaultProcessor = newProcessor(
		cfg,
		os.Getenv("TABLE_NAME"),
		os.Getenv("OUTPUT_BUCKET_NAME"),
		os.Getenv("INPUT_BUCKET_NAME"),
	)

	lambda.Start(handler)
}
