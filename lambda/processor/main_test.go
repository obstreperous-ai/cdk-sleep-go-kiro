package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	pollytypes "github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// --- Mock implementations ---

type mockS3Client struct {
	GetObjectFunc func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObjectFunc func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.GetObjectFunc != nil {
		return m.GetObjectFunc(ctx, params, optFns...)
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader("fake audio data")),
	}, nil
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.PutObjectFunc != nil {
		return m.PutObjectFunc(ctx, params, optFns...)
	}
	return &s3.PutObjectOutput{}, nil
}

type mockDynamoDBClient struct {
	UpdateItemFunc func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

func (m *mockDynamoDBClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if m.UpdateItemFunc != nil {
		return m.UpdateItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

type mockPollyClient struct {
	SynthesizeSpeechFunc func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error)
}

func (m *mockPollyClient) SynthesizeSpeech(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
	if m.SynthesizeSpeechFunc != nil {
		return m.SynthesizeSpeechFunc(ctx, params, optFns...)
	}
	return &polly.SynthesizeSpeechOutput{
		AudioStream:       io.NopCloser(strings.NewReader("synthesized audio bytes")),
		ContentType:       strPtr("audio/mpeg"),
		RequestCharacters: 62,
	}, nil
}

func strPtr(s string) *string {
	return &s
}

// --- Validation tests (existing behavior, no processor needed) ---

func TestHandler_ValidInput(t *testing.T) {
	// Ensure no processor is set so validation-only path is exercised
	oldProcessor := defaultProcessor
	defaultProcessor = nil
	defer func() { defaultProcessor = oldProcessor }()

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
	oldProcessor := defaultProcessor
	defaultProcessor = nil
	defer func() { defaultProcessor = oldProcessor }()

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
	oldProcessor := defaultProcessor
	defaultProcessor = nil
	defer func() { defaultProcessor = oldProcessor }()

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
	oldProcessor := defaultProcessor
	defaultProcessor = nil
	defer func() { defaultProcessor = oldProcessor }()

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
		wErr.Close()
		wOut.Close()
		os.Stderr = oldStderr
		os.Stdout = oldStdout
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

// --- Processing logic tests ---

func TestProcessor_SuccessfulProcessing(t *testing.T) {
	var putObjectCalled bool
	var putObjectBucket, putObjectKey string
	var updateItemCalled bool
	var updateItemTable string

	s3Mock := &mockS3Client{
		PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			putObjectCalled = true
			putObjectBucket = *params.Bucket
			putObjectKey = *params.Key
			return &s3.PutObjectOutput{}, nil
		},
	}

	dynamoMock := &mockDynamoDBClient{
		UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			updateItemCalled = true
			updateItemTable = *params.TableName
			return &dynamodb.UpdateItemOutput{}, nil
		},
	}

	pollyMock := &mockPollyClient{
		SynthesizeSpeechFunc: func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
			// Verify the Polly params
			if *params.Text != "Welcome to your sleep audio session. Relax and breathe deeply." {
				t.Errorf("unexpected Polly text: %s", *params.Text)
			}
			if params.VoiceId != pollytypes.VoiceIdJoanna {
				t.Errorf("expected VoiceId Joanna, got %v", params.VoiceId)
			}
			if params.OutputFormat != pollytypes.OutputFormatMp3 {
				t.Errorf("expected OutputFormat mp3, got %v", params.OutputFormat)
			}
			return &polly.SynthesizeSpeechOutput{
				AudioStream: io.NopCloser(strings.NewReader("synthesized audio bytes")),
			}, nil
		},
	}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "output-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	event := Event{
		AudioID:   "test-audio-123",
		Bucket:    "input-bucket",
		ObjectKey: "uploads/test-audio-123.mp3",
	}

	resp, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify response
	if resp.Status != "COMPLETED" {
		t.Errorf("expected status COMPLETED, got %s", resp.Status)
	}
	if resp.AudioID != "test-audio-123" {
		t.Errorf("expected audioId test-audio-123, got %s", resp.AudioID)
	}
	if !strings.HasPrefix(resp.OutputLocation, "s3://output-bucket/processed/test-audio-123/") {
		t.Errorf("unexpected OutputLocation: %s", resp.OutputLocation)
	}
	if !strings.HasSuffix(resp.OutputLocation, ".mp3") {
		t.Errorf("expected OutputLocation to end with .mp3: %s", resp.OutputLocation)
	}
	if resp.FileSize != int64(len("synthesized audio bytes")) {
		t.Errorf("expected FileSize %d, got %d", len("synthesized audio bytes"), resp.FileSize)
	}
	if resp.ProcessingDuration == "" {
		t.Error("expected ProcessingDuration to be set")
	}

	// Verify S3 PutObject was called with correct params
	if !putObjectCalled {
		t.Error("expected S3 PutObject to be called")
	}
	if putObjectBucket != "output-bucket" {
		t.Errorf("expected PutObject bucket 'output-bucket', got %s", putObjectBucket)
	}
	if !strings.HasPrefix(putObjectKey, "processed/test-audio-123/") {
		t.Errorf("unexpected PutObject key: %s", putObjectKey)
	}

	// Verify DynamoDB UpdateItem was called
	if !updateItemCalled {
		t.Error("expected DynamoDB UpdateItem to be called")
	}
	if updateItemTable != "audio-table" {
		t.Errorf("expected table 'audio-table', got %s", updateItemTable)
	}
}

func TestProcessor_S3GetObjectFailure(t *testing.T) {
	var dynamoUpdateCalled bool
	var dynamoStatus string

	s3Mock := &mockS3Client{
		GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	dynamoMock := &mockDynamoDBClient{
		UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			dynamoUpdateCalled = true
			if sv, ok := params.ExpressionAttributeValues[":status"].(*dynamodbtypes.AttributeValueMemberS); ok {
				dynamoStatus = sv.Value
			}
			return &dynamodb.UpdateItemOutput{}, nil
		},
	}

	pollyMock := &mockPollyClient{}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "output-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	event := Event{
		AudioID:   "test-audio-456",
		Bucket:    "input-bucket",
		ObjectKey: "uploads/test-audio-456.mp3",
	}

	_, err := handler(context.Background(), event)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to download from S3") {
		t.Errorf("expected error about S3 download failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected error to contain 'access denied', got: %v", err)
	}

	// Verify DynamoDB was called with FAILED status
	if !dynamoUpdateCalled {
		t.Error("expected DynamoDB UpdateItem to be called with FAILED status")
	}
	if dynamoStatus != "FAILED" {
		t.Errorf("expected DynamoDB status FAILED, got %s", dynamoStatus)
	}
}

func TestProcessor_PollySynthesizeFailure(t *testing.T) {
	var dynamoUpdateCalled bool
	var dynamoStatus string

	s3Mock := &mockS3Client{}

	dynamoMock := &mockDynamoDBClient{
		UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			dynamoUpdateCalled = true
			if sv, ok := params.ExpressionAttributeValues[":status"].(*dynamodbtypes.AttributeValueMemberS); ok {
				dynamoStatus = sv.Value
			}
			return &dynamodb.UpdateItemOutput{}, nil
		},
	}

	pollyMock := &mockPollyClient{
		SynthesizeSpeechFunc: func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
			return nil, fmt.Errorf("service unavailable")
		},
	}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "output-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	event := Event{
		AudioID:   "test-audio-789",
		Bucket:    "input-bucket",
		ObjectKey: "uploads/test-audio-789.wav",
	}

	_, err := handler(context.Background(), event)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to synthesize speech") {
		t.Errorf("expected error about Polly failure, got: %v", err)
	}

	// Verify graceful degradation: DynamoDB updated with FAILED status
	if !dynamoUpdateCalled {
		t.Error("expected DynamoDB UpdateItem to be called with FAILED status")
	}
	if dynamoStatus != "FAILED" {
		t.Errorf("expected DynamoDB status FAILED, got %s", dynamoStatus)
	}
}

func TestProcessor_S3PutObjectFailure(t *testing.T) {
	var dynamoUpdateCalled bool
	var dynamoStatus string

	s3Mock := &mockS3Client{
		PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			return nil, fmt.Errorf("bucket not found")
		},
	}

	dynamoMock := &mockDynamoDBClient{
		UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			dynamoUpdateCalled = true
			if sv, ok := params.ExpressionAttributeValues[":status"].(*dynamodbtypes.AttributeValueMemberS); ok {
				dynamoStatus = sv.Value
			}
			return &dynamodb.UpdateItemOutput{}, nil
		},
	}

	pollyMock := &mockPollyClient{}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "output-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	event := Event{
		AudioID:   "test-audio-upload-fail",
		Bucket:    "input-bucket",
		ObjectKey: "uploads/test.mp3",
	}

	_, err := handler(context.Background(), event)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to upload to S3") {
		t.Errorf("expected error about S3 upload failure, got: %v", err)
	}

	// Verify DynamoDB was called with FAILED status
	if !dynamoUpdateCalled {
		t.Error("expected DynamoDB UpdateItem to be called with FAILED status")
	}
	if dynamoStatus != "FAILED" {
		t.Errorf("expected DynamoDB status FAILED, got %s", dynamoStatus)
	}
}

func TestProcessor_DynamoDBUpdateFailure(t *testing.T) {
	s3Mock := &mockS3Client{}
	pollyMock := &mockPollyClient{}

	dynamoMock := &mockDynamoDBClient{
		UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			return nil, fmt.Errorf("provisioned throughput exceeded")
		},
	}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "output-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	event := Event{
		AudioID:   "test-audio-dynamo-fail",
		Bucket:    "input-bucket",
		ObjectKey: "uploads/test.flac",
	}

	resp, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error (DynamoDB failure on COMPLETED path is logged, not returned), got: %v", err)
	}
	// Processing is considered complete even if DynamoDB update fails
	if resp.Status != "COMPLETED" {
		t.Errorf("expected status COMPLETED, got %s", resp.Status)
	}
	if resp.AudioID != "test-audio-dynamo-fail" {
		t.Errorf("expected audioId test-audio-dynamo-fail, got %s", resp.AudioID)
	}
}

func TestProcessor_InputBucketValidation(t *testing.T) {
	s3Mock := &mockS3Client{}
	dynamoMock := &mockDynamoDBClient{}
	pollyMock := &mockPollyClient{}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "output-bucket",
		InputBucket:    "expected-input-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	t.Run("mismatched bucket returns error", func(t *testing.T) {
		event := Event{
			AudioID:   "bucket-mismatch-test",
			Bucket:    "wrong-bucket",
			ObjectKey: "uploads/test.mp3",
		}

		_, err := handler(context.Background(), event)
		if err == nil {
			t.Fatal("expected an error for mismatched bucket, got nil")
		}
		if !strings.Contains(err.Error(), "does not match configured input bucket") {
			t.Errorf("expected bucket mismatch error, got: %v", err)
		}
	})

	t.Run("matching bucket succeeds", func(t *testing.T) {
		event := Event{
			AudioID:   "bucket-match-test",
			Bucket:    "expected-input-bucket",
			ObjectKey: "uploads/test.mp3",
		}

		resp, err := handler(context.Background(), event)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if resp.Status != "COMPLETED" {
			t.Errorf("expected status COMPLETED, got %s", resp.Status)
		}
	})

	t.Run("empty InputBucket skips validation", func(t *testing.T) {
		procNoInputBucket := &Processor{
			S3Client:       s3Mock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyMock,
			TableName:      "audio-table",
			OutputBucket:   "output-bucket",
			InputBucket:    "",
		}
		defaultProcessor = procNoInputBucket

		event := Event{
			AudioID:   "no-input-bucket-test",
			Bucket:    "any-bucket",
			ObjectKey: "uploads/test.mp3",
		}

		resp, err := handler(context.Background(), event)
		if err != nil {
			t.Fatalf("expected no error when InputBucket is empty, got: %v", err)
		}
		if resp.Status != "COMPLETED" {
			t.Errorf("expected status COMPLETED, got %s", resp.Status)
		}
	})
}

func TestProcessor_ResponseContainsOutputFields(t *testing.T) {
	s3Mock := &mockS3Client{}
	dynamoMock := &mockDynamoDBClient{}
	pollyMock := &mockPollyClient{
		SynthesizeSpeechFunc: func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
			return &polly.SynthesizeSpeechOutput{
				AudioStream: io.NopCloser(strings.NewReader("audio data with known size")),
			}, nil
		},
	}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "my-output-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	event := Event{
		AudioID:   "response-test",
		Bucket:    "input-bucket",
		ObjectKey: "uploads/response-test.mp3",
	}

	startBefore := time.Now()
	resp, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify OutputLocation contains the S3 URI with correct bucket and pattern
	if !strings.Contains(resp.OutputLocation, "s3://my-output-bucket/processed/response-test/") {
		t.Errorf("expected OutputLocation to contain S3 URI pattern, got: %s", resp.OutputLocation)
	}

	// Verify FileSize matches the known audio data
	expectedSize := int64(len("audio data with known size"))
	if resp.FileSize != expectedSize {
		t.Errorf("expected FileSize %d, got %d", expectedSize, resp.FileSize)
	}

	// Verify ProcessingDuration is set and represents a valid duration
	if resp.ProcessingDuration == "" {
		t.Error("expected ProcessingDuration to be set")
	}
	duration, err := time.ParseDuration(resp.ProcessingDuration)
	if err != nil {
		t.Errorf("expected ProcessingDuration to be parseable, got: %s", resp.ProcessingDuration)
	}
	if duration < 0 || duration > time.Since(startBefore)+time.Second {
		t.Errorf("ProcessingDuration %v seems unreasonable", duration)
	}
}

func TestProcessor_PollyCallsWithCorrectParams(t *testing.T) {
	var pollyText string
	var pollyVoice pollytypes.VoiceId
	var pollyFormat pollytypes.OutputFormat

	s3Mock := &mockS3Client{}
	dynamoMock := &mockDynamoDBClient{}
	pollyMock := &mockPollyClient{
		SynthesizeSpeechFunc: func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
			pollyText = *params.Text
			pollyVoice = params.VoiceId
			pollyFormat = params.OutputFormat
			return &polly.SynthesizeSpeechOutput{
				AudioStream: io.NopCloser(strings.NewReader("audio")),
			}, nil
		},
	}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "output-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	event := Event{
		AudioID:   "polly-params-test",
		Bucket:    "input-bucket",
		ObjectKey: "uploads/polly-params-test.mp3",
	}

	_, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if pollyText != "Welcome to your sleep audio session. Relax and breathe deeply." {
		t.Errorf("unexpected Polly text: %s", pollyText)
	}
	if pollyVoice != pollytypes.VoiceIdJoanna {
		t.Errorf("expected VoiceId Joanna, got %v", pollyVoice)
	}
	if pollyFormat != pollytypes.OutputFormatMp3 {
		t.Errorf("expected OutputFormat mp3, got %v", pollyFormat)
	}
}

func TestProcessor_S3GetObjectCalledWithCorrectBucket(t *testing.T) {
	var getObjectBucket, getObjectKey string

	s3Mock := &mockS3Client{
		GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			getObjectBucket = *params.Bucket
			getObjectKey = *params.Key
			return &s3.GetObjectOutput{
				Body: io.NopCloser(strings.NewReader("audio data")),
			}, nil
		},
	}
	dynamoMock := &mockDynamoDBClient{}
	pollyMock := &mockPollyClient{}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "output-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	event := Event{
		AudioID:   "bucket-test",
		Bucket:    "my-input-bucket",
		ObjectKey: "path/to/audio.mp3",
	}

	_, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if getObjectBucket != "my-input-bucket" {
		t.Errorf("expected GetObject bucket 'my-input-bucket', got %s", getObjectBucket)
	}
	if getObjectKey != "path/to/audio.mp3" {
		t.Errorf("expected GetObject key 'path/to/audio.mp3', got %s", getObjectKey)
	}
}

func TestProcessor_DynamoDBUpdateContainsExpectedFields(t *testing.T) {
	var updateExpression string
	var expressionNames map[string]string

	s3Mock := &mockS3Client{}
	pollyMock := &mockPollyClient{}
	dynamoMock := &mockDynamoDBClient{
		UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			updateExpression = *params.UpdateExpression
			expressionNames = params.ExpressionAttributeNames
			return &dynamodb.UpdateItemOutput{}, nil
		},
	}

	proc := &Processor{
		S3Client:       s3Mock,
		DynamoDBClient: dynamoMock,
		PollyClient:    pollyMock,
		TableName:      "audio-table",
		OutputBucket:   "output-bucket",
	}

	oldProcessor := defaultProcessor
	defaultProcessor = proc
	defer func() { defaultProcessor = oldProcessor }()

	event := Event{
		AudioID:   "dynamo-fields-test",
		Bucket:    "input-bucket",
		ObjectKey: "uploads/dynamo-fields-test.mp3",
	}

	_, err := handler(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify update expression contains status, updatedAt, outputLocation, fileSize
	if !strings.Contains(updateExpression, "#s = :status") {
		t.Errorf("expected update expression to contain status, got: %s", updateExpression)
	}
	if !strings.Contains(updateExpression, "#ua = :updatedAt") {
		t.Errorf("expected update expression to contain updatedAt, got: %s", updateExpression)
	}
	if !strings.Contains(updateExpression, "#ol = :outputLocation") {
		t.Errorf("expected update expression to contain outputLocation, got: %s", updateExpression)
	}
	if !strings.Contains(updateExpression, "#fs = :fileSize") {
		t.Errorf("expected update expression to contain fileSize, got: %s", updateExpression)
	}

	// Verify expression attribute names
	if expressionNames["#s"] != "status" {
		t.Errorf("expected #s to be 'status', got %s", expressionNames["#s"])
	}
	if expressionNames["#ol"] != "outputLocation" {
		t.Errorf("expected #ol to be 'outputLocation', got %s", expressionNames["#ol"])
	}
}

// --- End-to-End Processor Integration Tests ---

func TestEndToEndProcessorIntegration(t *testing.T) {
	// Validates the full processor pipeline flow end-to-end:
	// S3 download -> Polly synthesis -> S3 upload -> DynamoDB update

	t.Run("FullSuccessPath", func(t *testing.T) {
		// Track all service calls in order
		var callOrder []string
		var finalDynamoStatus string
		var finalOutputLocation string

		s3Mock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				callOrder = append(callOrder, "s3:GetObject")
				if *params.Bucket != "input-bucket" {
					t.Errorf("GetObject called with wrong bucket: %s", *params.Bucket)
				}
				if *params.Key != "uploads/e2e-test.mp3" {
					t.Errorf("GetObject called with wrong key: %s", *params.Key)
				}
				return &s3.GetObjectOutput{
					Body: io.NopCloser(strings.NewReader("real audio content")),
				}, nil
			},
			PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
				callOrder = append(callOrder, "s3:PutObject")
				if *params.Bucket != "output-bucket" {
					t.Errorf("PutObject called with wrong bucket: %s", *params.Bucket)
				}
				if !strings.HasPrefix(*params.Key, "processed/e2e-audio-id/") {
					t.Errorf("PutObject key doesn't match expected pattern: %s", *params.Key)
				}
				return &s3.PutObjectOutput{}, nil
			},
		}

		dynamoMock := &mockDynamoDBClient{
			UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				callOrder = append(callOrder, "dynamodb:UpdateItem")
				if sv, ok := params.ExpressionAttributeValues[":status"].(*dynamodbtypes.AttributeValueMemberS); ok {
					finalDynamoStatus = sv.Value
				}
				if ol, ok := params.ExpressionAttributeValues[":outputLocation"].(*dynamodbtypes.AttributeValueMemberS); ok {
					finalOutputLocation = ol.Value
				}
				return &dynamodb.UpdateItemOutput{}, nil
			},
		}

		pollyMock := &mockPollyClient{
			SynthesizeSpeechFunc: func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
				callOrder = append(callOrder, "polly:SynthesizeSpeech")
				return &polly.SynthesizeSpeechOutput{
					AudioStream: io.NopCloser(strings.NewReader("polly-synthesized-audio")),
				}, nil
			},
		}

		proc := &Processor{
			S3Client:       s3Mock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyMock,
			TableName:      "e2e-table",
			OutputBucket:   "output-bucket",
		}

		oldProcessor := defaultProcessor
		defaultProcessor = proc
		defer func() { defaultProcessor = oldProcessor }()

		event := Event{
			AudioID:   "e2e-audio-id",
			Bucket:    "input-bucket",
			ObjectKey: "uploads/e2e-test.mp3",
		}

		resp, err := handler(context.Background(), event)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify final response
		if resp.Status != "COMPLETED" {
			t.Errorf("expected COMPLETED status, got %s", resp.Status)
		}
		if resp.AudioID != "e2e-audio-id" {
			t.Errorf("expected audioId 'e2e-audio-id', got %s", resp.AudioID)
		}
		if !strings.Contains(resp.OutputLocation, "s3://output-bucket/processed/e2e-audio-id/") {
			t.Errorf("unexpected OutputLocation: %s", resp.OutputLocation)
		}

		// Verify DynamoDB was updated with COMPLETED and outputLocation
		if finalDynamoStatus != "COMPLETED" {
			t.Errorf("expected DynamoDB status COMPLETED, got %s", finalDynamoStatus)
		}
		if !strings.Contains(finalOutputLocation, "s3://output-bucket/processed/e2e-audio-id/") {
			t.Errorf("expected DynamoDB outputLocation to contain S3 URI, got %s", finalOutputLocation)
		}

		// Verify call order: GetObject -> SynthesizeSpeech -> PutObject -> UpdateItem
		expectedOrder := []string{"s3:GetObject", "polly:SynthesizeSpeech", "s3:PutObject", "dynamodb:UpdateItem"}
		if len(callOrder) != len(expectedOrder) {
			t.Fatalf("expected %d service calls, got %d: %v", len(expectedOrder), len(callOrder), callOrder)
		}
		for i, expected := range expectedOrder {
			if callOrder[i] != expected {
				t.Errorf("call order[%d]: expected %s, got %s (full order: %v)", i, expected, callOrder[i], callOrder)
			}
		}
	})

	t.Run("InvalidInputRejected", func(t *testing.T) {
		// Invalid file extension should be rejected before any service calls
		s3Mock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Error("S3 GetObject should not be called for invalid input")
				return nil, fmt.Errorf("should not be called")
			},
		}
		dynamoMock := &mockDynamoDBClient{
			UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				t.Error("DynamoDB should not be called for invalid input")
				return nil, fmt.Errorf("should not be called")
			},
		}
		pollyMock := &mockPollyClient{
			SynthesizeSpeechFunc: func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
				t.Error("Polly should not be called for invalid input")
				return nil, fmt.Errorf("should not be called")
			},
		}

		proc := &Processor{
			S3Client:       s3Mock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyMock,
			TableName:      "e2e-table",
			OutputBucket:   "output-bucket",
		}

		oldProcessor := defaultProcessor
		defaultProcessor = proc
		defer func() { defaultProcessor = oldProcessor }()

		event := Event{
			AudioID:   "invalid-file",
			Bucket:    "input-bucket",
			ObjectKey: "uploads/invalid-file.txt",
		}

		_, err := handler(context.Background(), event)
		if err == nil {
			t.Fatal("expected an error for invalid extension")
		}
		if !strings.Contains(err.Error(), "unsupported file extension") {
			t.Errorf("expected unsupported file extension error, got: %v", err)
		}
	})

	t.Run("S3ErrorCausesDynamoDBFailedUpdate", func(t *testing.T) {
		var dynamoStatus string

		s3Mock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return nil, fmt.Errorf("NoSuchKey: The specified key does not exist")
			},
		}
		dynamoMock := &mockDynamoDBClient{
			UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				if sv, ok := params.ExpressionAttributeValues[":status"].(*dynamodbtypes.AttributeValueMemberS); ok {
					dynamoStatus = sv.Value
				}
				return &dynamodb.UpdateItemOutput{}, nil
			},
		}
		pollyMock := &mockPollyClient{}

		proc := &Processor{
			S3Client:       s3Mock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyMock,
			TableName:      "e2e-table",
			OutputBucket:   "output-bucket",
		}

		oldProcessor := defaultProcessor
		defaultProcessor = proc
		defer func() { defaultProcessor = oldProcessor }()

		event := Event{
			AudioID:   "s3-error-test",
			Bucket:    "input-bucket",
			ObjectKey: "uploads/missing-file.mp3",
		}

		_, err := handler(context.Background(), event)
		if err == nil {
			t.Fatal("expected an error for S3 failure")
		}

		// DynamoDB should be updated with FAILED status
		if dynamoStatus != "FAILED" {
			t.Errorf("expected DynamoDB status FAILED, got %s", dynamoStatus)
		}
	})

	t.Run("PollyErrorCausesDynamoDBFailedUpdate", func(t *testing.T) {
		var dynamoStatus string

		s3Mock := &mockS3Client{}
		dynamoMock := &mockDynamoDBClient{
			UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				if sv, ok := params.ExpressionAttributeValues[":status"].(*dynamodbtypes.AttributeValueMemberS); ok {
					dynamoStatus = sv.Value
				}
				return &dynamodb.UpdateItemOutput{}, nil
			},
		}
		pollyMock := &mockPollyClient{
			SynthesizeSpeechFunc: func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
				return nil, fmt.Errorf("ThrottlingException: Rate exceeded")
			},
		}

		proc := &Processor{
			S3Client:       s3Mock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyMock,
			TableName:      "e2e-table",
			OutputBucket:   "output-bucket",
		}

		oldProcessor := defaultProcessor
		defaultProcessor = proc
		defer func() { defaultProcessor = oldProcessor }()

		event := Event{
			AudioID:   "polly-error-test",
			Bucket:    "input-bucket",
			ObjectKey: "uploads/polly-error.wav",
		}

		_, err := handler(context.Background(), event)
		if err == nil {
			t.Fatal("expected an error for Polly failure")
		}

		// DynamoDB should be updated with FAILED status (graceful degradation)
		if dynamoStatus != "FAILED" {
			t.Errorf("expected DynamoDB status FAILED after Polly error, got %s", dynamoStatus)
		}
	})
}

// --- Retry Behavior Tests ---

func TestRetryBehaviorUnderFailure(t *testing.T) {
	// Simulates transient failures (first call fails, second succeeds) to verify
	// the processor's error handling is compatible with Step Functions retry configuration.
	// Step Functions retries invoke the entire Lambda again, so each invocation is independent.

	t.Run("TransientS3FailureThenSuccess", func(t *testing.T) {
		// First invocation: S3 GetObject fails (simulating transient network error)
		s3FailMock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return nil, fmt.Errorf("RequestTimeout: request timed out")
			},
		}
		dynamoMock := &mockDynamoDBClient{}
		pollyMock := &mockPollyClient{}

		proc := &Processor{
			S3Client:       s3FailMock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyMock,
			TableName:      "retry-table",
			OutputBucket:   "output-bucket",
		}

		oldProcessor := defaultProcessor
		defaultProcessor = proc
		defer func() { defaultProcessor = oldProcessor }()

		event := Event{
			AudioID:   "retry-test-s3",
			Bucket:    "input-bucket",
			ObjectKey: "uploads/retry-test.mp3",
		}

		// First invocation fails - returns an error that Step Functions can retry
		_, err := handler(context.Background(), event)
		if err == nil {
			t.Fatal("expected first invocation to return an error")
		}

		// Second invocation succeeds (simulating Step Functions retry)
		s3SuccessMock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(strings.NewReader("audio data on retry")),
				}, nil
			},
		}

		proc2 := &Processor{
			S3Client:       s3SuccessMock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyMock,
			TableName:      "retry-table",
			OutputBucket:   "output-bucket",
		}
		defaultProcessor = proc2

		resp, err := handler(context.Background(), event)
		if err != nil {
			t.Fatalf("expected second invocation to succeed, got: %v", err)
		}
		if resp.Status != "COMPLETED" {
			t.Errorf("expected COMPLETED on retry, got %s", resp.Status)
		}
	})

	t.Run("TransientPollyFailureThenSuccess", func(t *testing.T) {
		s3Mock := &mockS3Client{}
		dynamoMock := &mockDynamoDBClient{}

		// First invocation: Polly fails
		pollyFailMock := &mockPollyClient{
			SynthesizeSpeechFunc: func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
				return nil, fmt.Errorf("ServiceUnavailableException: service is temporarily unavailable")
			},
		}

		proc := &Processor{
			S3Client:       s3Mock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyFailMock,
			TableName:      "retry-table",
			OutputBucket:   "output-bucket",
		}

		oldProcessor := defaultProcessor
		defaultProcessor = proc
		defer func() { defaultProcessor = oldProcessor }()

		event := Event{
			AudioID:   "retry-test-polly",
			Bucket:    "input-bucket",
			ObjectKey: "uploads/retry-polly.mp3",
		}

		// First invocation fails
		_, err := handler(context.Background(), event)
		if err == nil {
			t.Fatal("expected first invocation to return an error")
		}

		// Second invocation succeeds (Step Functions retry)
		pollySuccessMock := &mockPollyClient{
			SynthesizeSpeechFunc: func(ctx context.Context, params *polly.SynthesizeSpeechInput, optFns ...func(*polly.Options)) (*polly.SynthesizeSpeechOutput, error) {
				return &polly.SynthesizeSpeechOutput{
					AudioStream: io.NopCloser(strings.NewReader("polly audio on retry")),
				}, nil
			},
		}

		proc2 := &Processor{
			S3Client:       s3Mock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollySuccessMock,
			TableName:      "retry-table",
			OutputBucket:   "output-bucket",
		}
		defaultProcessor = proc2

		resp, err := handler(context.Background(), event)
		if err != nil {
			t.Fatalf("expected second invocation to succeed, got: %v", err)
		}
		if resp.Status != "COMPLETED" {
			t.Errorf("expected COMPLETED on retry, got %s", resp.Status)
		}
	})

	t.Run("ErrorMessageIsRetryable", func(t *testing.T) {
		// Verify that errors returned by the processor are plain errors (not wrapped
		// in a way that would prevent Step Functions from matching error conditions).
		s3Mock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return nil, fmt.Errorf("InternalServerError: internal server error")
			},
		}
		dynamoMock := &mockDynamoDBClient{}
		pollyMock := &mockPollyClient{}

		proc := &Processor{
			S3Client:       s3Mock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyMock,
			TableName:      "retry-table",
			OutputBucket:   "output-bucket",
		}

		oldProcessor := defaultProcessor
		defaultProcessor = proc
		defer func() { defaultProcessor = oldProcessor }()

		event := Event{
			AudioID:   "retryable-error-test",
			Bucket:    "input-bucket",
			ObjectKey: "uploads/retryable.mp3",
		}

		_, err := handler(context.Background(), event)
		if err == nil {
			t.Fatal("expected an error")
		}

		// Error should be a plain error string that Step Functions can catch
		// and match against States.TaskFailed or Lambda.ServiceException
		errStr := err.Error()
		if errStr == "" {
			t.Error("error message should not be empty")
		}
		// The error should contain context about what failed
		if !strings.Contains(errStr, "failed to download from S3") {
			t.Errorf("error should describe the failure, got: %s", errStr)
		}
	})

	t.Run("IndependentInvocationsDoNotShareState", func(t *testing.T) {
		// Each Lambda invocation is independent (Step Functions creates new invocations
		// on retry). Verify that two separate handler calls with different audioIDs
		// produce different output paths, confirming no shared mutable state.
		var firstCallOutputKey string
		var secondCallOutputKey string

		callCount := 0

		s3Mock := &mockS3Client{
			PutObjectFunc: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
				callCount++
				if callCount == 1 {
					firstCallOutputKey = *params.Key
				} else {
					secondCallOutputKey = *params.Key
				}
				return &s3.PutObjectOutput{}, nil
			},
		}
		dynamoMock := &mockDynamoDBClient{}
		pollyMock := &mockPollyClient{}

		proc := &Processor{
			S3Client:       s3Mock,
			DynamoDBClient: dynamoMock,
			PollyClient:    pollyMock,
			TableName:      "retry-table",
			OutputBucket:   "output-bucket",
		}

		oldProcessor := defaultProcessor
		defaultProcessor = proc
		defer func() { defaultProcessor = oldProcessor }()

		// Use different audioIDs to confirm output keys are based on input, not shared state
		event1 := Event{
			AudioID:   "independent-state-test-1",
			Bucket:    "input-bucket",
			ObjectKey: "uploads/state-test-1.mp3",
		}
		event2 := Event{
			AudioID:   "independent-state-test-2",
			Bucket:    "input-bucket",
			ObjectKey: "uploads/state-test-2.mp3",
		}

		_, err1 := handler(context.Background(), event1)
		if err1 != nil {
			t.Fatalf("first invocation failed: %v", err1)
		}

		_, err2 := handler(context.Background(), event2)
		if err2 != nil {
			t.Fatalf("second invocation failed: %v", err2)
		}

		// Output keys should differ because audioIDs differ
		if firstCallOutputKey == "" || secondCallOutputKey == "" {
			t.Fatal("expected both invocations to call PutObject")
		}
		if firstCallOutputKey == secondCallOutputKey {
			t.Errorf("expected different output keys for different audioIDs, both got: %s", firstCallOutputKey)
		}
	})
}
