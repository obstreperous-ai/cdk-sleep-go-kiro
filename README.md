# CDK Sleep Audio Pipeline

This is an event-driven sleep audio processing pipeline built with AWS CDK in Go. It uses S3, EventBridge, Lambda, DynamoDB, and SNS to process uploaded sleep audio recordings.

## Strict TDD Rules

Write failing tests first, then minimal code. Run `go test -v ./...` and `cdk synth` before every commit.

## Useful Commands

- `go test -v ./...` - run unit tests
- `cdk synth` - synthesize CloudFormation
- `cdk deploy` - deploy stack
- `cdk diff` - compare with deployed
