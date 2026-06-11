# Contributing

## Architecture as Source of Truth

[`ARCHITECTURE.md`](./ARCHITECTURE.md) is the **single source of truth** for the Event-Driven Sleep Audio Pipeline design.

Before opening an issue or submitting a pull request that changes infrastructure or data flow, read `ARCHITECTURE.md` to understand the intended design. Any change that adds, removes, or modifies AWS resources, data flow steps, security controls, or observability components **must** include a corresponding update to `ARCHITECTURE.md` and its Mermaid diagram in the same commit or pull request. Reviewers will check for consistency between code and documentation.

If you are unsure whether your change affects the architecture, err on the side of updating the document.

---

## Development Environment Setup

### Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.25+ | CDK app and Lambda processor |
| Node.js | 22+ | AWS CDK CLI runtime |
| AWS CDK CLI | latest | `npm install -g aws-cdk` |
| AWS CLI | v2 | Account configuration and credential management |

### Initial setup

```bash
# Clone the repository
git clone https://github.com/obstreperous-ai/cdk-sleep-go-kiro.git
cd cdk-sleep-go-kiro

# Download CDK app dependencies
go mod download

# Download Lambda processor dependencies
cd lambda/processor && go mod download && cd ../..

# Install the CDK CLI
npm install -g aws-cdk

# Verify setup
go test -v -count=1 ./...
cdk synth
```

### Environment variables

When running locally, you may need to set these environment variables if you encounter issues:

```bash
# Avoid Node.js proxy-bootstrap issues in sandboxed environments
export NODE_OPTIONS=''

# Ensure Go modules resolve correctly
export GOPROXY=https://proxy.golang.org,direct
```

---

## Testing Strategy

The project follows **strict TDD** (Test-Driven Development): write failing tests first, then write the minimal code to make them pass. No production code is merged without a corresponding test.

### Test layers

| Layer | File(s) | What it tests |
|---|---|---|
| CDK assertions | `cdk-base_test.go` | Infrastructure resources exist with correct properties |
| IAM permissions | `cdk-base_test.go` | Least-privilege policies are scoped correctly |
| E2E validation | `cdk-base_test.go` | Full pipeline wiring from S3 event through to SNS notification |
| Snapshot stability | `cdk-base_test.go` | Golden file comparison prevents unintended infrastructure drift |
| Lambda unit tests | `lambda/processor/main_test.go` | Handler logic, validation, error handling with mocked AWS services |
| Lambda integration | `lambda/processor/main_test.go` | End-to-end processor flow and retry behavior |
| Pipeline tests | `pipeline_test.go` | CDK Pipeline stack synthesizes correctly |

### Running tests

```bash
# All tests
go test -v -count=1 ./...

# CDK infrastructure tests only
go test -v -count=1 -run TestStack ./

# Lambda processor tests only
go test -v -count=1 ./lambda/processor/

# Specific test
go test -v -count=1 -run TestEndToEndPipelineValidation ./
```

### Snapshot test

The snapshot test (`TestStackSnapshotStability`) compares the synthesized CloudFormation template against a golden file at `testdata/snapshot.json`. If you intentionally change infrastructure:

1. Delete `testdata/snapshot.json`
2. Run the snapshot test to regenerate: `go test -v -count=1 -run TestStackSnapshotStability ./`
3. Review the generated snapshot to confirm the changes are correct
4. Commit the updated snapshot alongside your code changes

### Mock pattern (Lambda tests)

The Lambda tests use interface-based mocks for AWS SDK clients:

```go
type mockS3Client struct {
    GetObjectFunc func(ctx context.Context, params *s3.GetObjectInput, ...) (*s3.GetObjectOutput, error)
    PutObjectFunc func(ctx context.Context, params *s3.PutObjectInput, ...) (*s3.PutObjectOutput, error)
}
```

When adding new AWS service interactions to the Lambda, define a new interface and corresponding mock struct following this pattern.

---

## Code Structure

### Root CDK application

| File | Purpose |
|---|---|
| `cdk-base.go` | Main infrastructure stack: S3 buckets, EventBridge rule, Step Functions state machine, DynamoDB table, Lambda function, SNS topics, CloudWatch alarms |
| `cdk-base_test.go` | CDK assertion tests, E2E validation tests, snapshot test |
| `pipeline.go` | CDK Pipelines CI/CD stack (conditionally instantiated) |
| `pipeline_test.go` | Pipeline stack tests |
| `cdk.json` | CDK app entry point and feature flags |

### Lambda processor

| File | Purpose |
|---|---|
| `lambda/processor/main.go` | Handler: validates input, downloads from S3, calls Polly, uploads to S3, updates DynamoDB |
| `lambda/processor/main_test.go` | Unit tests with mocked S3, Polly, and DynamoDB clients |
| `lambda/processor/go.mod` | Separate Go module for Lambda (isolated AWS SDK dependencies) |

### CI/CD

| File | Purpose |
|---|---|
| `.github/workflows/ci.yml` | GitHub Actions: test + synth on push/PR |
| `pipeline.go` | AWS CodePipeline via CDK Pipelines |

---

## How to Extend the Pipeline

### Adding a new processing step

1. **Write a failing test** in `cdk-base_test.go` that asserts the new state exists in the state machine definition
2. **Add the state** in `cdk-base.go` within the Step Functions chain
3. **Update error handling** - add appropriate Retry and Catch blocks
4. **Update permissions** - grant the state machine role any new IAM actions
5. **Run tests** and regenerate the snapshot: `rm testdata/snapshot.json && go test -v -count=1 ./...`
6. **Update ARCHITECTURE.md** with the new state in the Orchestration Layer section and update Mermaid diagrams

### Adding a new Lambda function

1. Create a new directory under `lambda/` (e.g., `lambda/newfunction/`)
2. Initialize a Go module: `cd lambda/newfunction && go mod init newfunction`
3. Follow the same handler pattern as `lambda/processor/main.go` (interface-based clients, structured logging)
4. Write unit tests with mocked AWS service clients
5. Add the Lambda construct in `cdk-base.go` with appropriate IAM permissions
6. Wire it into the Step Functions state machine

### Modifying the state machine flow

1. Write a failing E2E validation test that asserts the new flow
2. Modify the state chain in `cdk-base.go`
3. Update Catch/Retry blocks as needed
4. Ensure the Current State Diagram in `ARCHITECTURE.md` reflects the change
5. Regenerate the snapshot

---

## Conventional Commits

All commit messages must follow the Conventional Commits format:

- `feat:` - new feature
- `fix:` - bug fix
- `chore:` - maintenance tasks
- `docs:` - documentation changes
- `refactor:` - code restructuring without behavior change

---

## Before Pushing

Run the following commands and ensure they pass:

```bash
# Run all tests
go test -v -count=1 ./...

# Synthesize CloudFormation (validates CDK app compiles and produces valid output)
cdk synth
```

Both checks run in CI via GitHub Actions. Failing either will block pull request merges.
