# Project Summary

## Overview

The **Event-Driven Sleep Audio Pipeline** is a serverless AWS system that accepts raw audio uploads and transforms them into polished sleep audio artifacts. The pipeline is fully event-driven, using S3 events to trigger an orchestrated processing workflow that validates input, invokes a Go Lambda for audio processing, synthesizes speech via Amazon Polly, and delivers results with full metadata tracking and notifications.

---

## Key Architectural Decisions

| Decision | Rationale |
|---|---|
| **Serverless-only architecture** | Zero idle cost, automatic scaling with upload volume, minimal operational overhead |
| **Event-driven via EventBridge** | Decouples ingestion from processing; native S3 integration; supports filtering and fan-out |
| **Step Functions Express Workflows** | Built-in retry/catch logic, visual debugging, cost-effective for short-duration jobs (under 60 seconds) |
| **Go for CDK and Lambda** | Type safety, fast cold starts on Lambda (custom runtime `provided.al2023`), single language for infra and application code |
| **Separate Go modules** | Lambda processor has its own `go.mod` to isolate AWS SDK dependencies from CDK dependencies, reducing deployment artifact size |
| **DynamoDB for metadata** | Single-digit millisecond latency, serverless scaling, simple key design for job lookups |
| **Two SNS topics (completed/failed)** | Separate notification channels allow distinct downstream routing for success and failure events |
| **Defense-in-depth validation** | Step Functions Choice state provides fast-fail for invalid extensions; Lambda performs secondary validation including required-field checks |
| **CDK context for multi-environment** | Single codebase deploys to dev/staging/prod with environment-specific configuration via `-c env=X` |
| **CDK Pipelines for CI/CD** | Self-mutating pipeline ensures infrastructure-as-code changes are automatically deployed |

---

## What Was Built

### Infrastructure Components

- **S3 Input Bucket** - Encrypted, versioned, EventBridge-enabled, block public access
- **S3 Output Bucket** - Encrypted, versioned, block public access
- **EventBridge Rule** - Matches S3 ObjectCreated events, targets Step Functions
- **Step Functions State Machine** - 6-state pipeline with error handling, retries, and X-Ray tracing
- **Lambda Function (SleepAudioProcessor)** - Go custom runtime, full audio processing pipeline
- **DynamoDB Table** - On-demand billing, point-in-time recovery, AWS-managed encryption
- **SNS Topic (Completed)** - KMS-encrypted success notifications
- **SNS Topic (Failed)** - KMS-encrypted failure notifications
- **CloudWatch Log Group** - State machine execution logs at ALL level
- **CloudWatch Alarms** - ExecutionsFailed and Lambda Errors monitoring with SNS alerting

### State Machine Flow

```
WriteInitialRecord -> ValidateInput -> [valid] ProcessAudio -> PollyTask -> MarkCompleted -> NotifyCompleted
                                    -> [invalid] MarkFailed -> NotifyFailed
                      ProcessAudio/PollyTask errors -> MarkFailed -> NotifyFailed
```

### Lambda Processor Capabilities

- Input validation (required fields + file extension)
- S3 download from input bucket
- Amazon Polly speech synthesis (voice: Joanna, format: mp3)
- S3 upload to output bucket with structured key pattern
- DynamoDB metadata update with status, output location, file size
- Structured JSON logging for CloudWatch Logs Insights
- Graceful error handling with DynamoDB failure recording

### CI/CD

- GitHub Actions workflow (test + synth on push/PR)
- CDK Pipelines skeleton (CodePipeline, conditionally instantiated)

### Test Suite

- CDK assertion tests for all infrastructure resources
- IAM permission verification tests
- End-to-end pipeline validation tests (success + failure paths)
- Snapshot stability test (golden file comparison)
- Lambda unit tests with mocked AWS services
- Lambda integration tests (full processing flow)
- Retry behavior tests (transient failure recovery)

---

## TDD Process

The project was built following strict Test-Driven Development throughout:

1. **Write a failing test** - Define the expected behavior before writing any implementation
2. **Implement minimal code** - Write just enough code to make the test pass
3. **Refactor** - Clean up while keeping tests green
4. **Repeat** - Each new feature starts with a new failing test

### TDD at each layer

- **Infrastructure (CDK assertions)** - Tests assert specific CloudFormation resources and properties exist before the CDK constructs are written
- **Lambda handler (unit tests)** - Tests define expected behavior with mock clients before the handler logic is implemented
- **Integration (E2E tests)** - Tests validate cross-resource wiring before the connections are made
- **Snapshot (regression)** - Golden file locks the infrastructure state, requiring explicit regeneration for any change

### Benefits observed

- Caught IAM permission issues early (CDK `GrantRead` generates wildcarded actions like `s3:GetObject*`, not specific `s3:GetObject`)
- Snapshot test prevented unintended infrastructure drift during refactoring
- Mock-based Lambda tests enabled rapid iteration without AWS credentials
- E2E validation tests verified the full state machine chain without deployment

---

## Experiment Notes

### Lessons Learned

1. **CDK feature flags matter** - The `@aws-cdk/aws-iam:minimizePolicies` flag merges IAM policy statements into the role's default policy rather than creating dedicated policy resources. Tests must account for this structure.

2. **Step Functions retry configuration** - Retries are specified per-state, not globally. Each task needs its own retry policy with appropriate error type matching. Using specific error types first with a `States.ALL` fallback provides the best balance of control and safety.

3. **Dual-validation pattern** - Validating at both the Step Functions level (Choice state for file extension) and Lambda level (required fields + extension) provides defense-in-depth. The Step Functions check is cheaper (no compute invoked) while the Lambda check covers more cases.

4. **Express vs Standard Workflows** - Express Workflows are appropriate here because audio processing jobs complete in under 60 seconds. They offer higher throughput and lower cost but lack visual execution history in the console (logs provide equivalent visibility).

5. **Go CDK patterns** - The Go CDK bindings use `jsii.String()` wrappers extensively. State machine definitions use chaining (`Next()`) with explicit Pass/Fail states rather than implicit transitions.

6. **Snapshot test maintenance** - The snapshot golden file must be deleted and regenerated whenever infrastructure changes are made. Automated regeneration in CI is not recommended since the snapshot exists to catch unintended changes.

7. **Lambda output key format** - Using second-level timestamp precision (`20060102T150405Z`) in S3 output keys provides sufficient uniqueness for the expected throughput while maintaining human readability.

### Trade-offs

| Trade-off | Chosen approach | Alternative considered |
|---|---|---|
| Workflow type | Express (high throughput, low cost) | Standard (visual history, longer duration support) |
| Lambda runtime | Go custom runtime (fast cold start) | Node.js/Python (richer CDK integration) |
| Error notification | Separate SNS topics per outcome | Single topic with message attributes for filtering |
| Metadata storage | DynamoDB (serverless, fast) | Aurora Serverless (relational queries) |
| Audio enhancement | Polly TTS (managed, predictable) | Bedrock AI models (more flexible, higher cost) |
| Deployment model | CDK Pipelines (self-mutating) | GitHub Actions deploy (simpler, external) |
