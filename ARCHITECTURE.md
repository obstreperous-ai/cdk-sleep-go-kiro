# Architecture

> This document is the **source of truth** for the Event-Driven Sleep Audio Pipeline design.
> All infrastructure changes must be reflected here before (or alongside) implementation.

---

## Table of Contents

1. [High-Level Overview](#high-level-overview)
2. [Data Flow](#data-flow)
3. [System Diagram](#system-diagram)
4. [AWS Services and Rationale](#aws-services-and-rationale)
5. [Security Design](#security-design)
6. [Observability](#observability)
7. [Multi-Environment Support](#multi-environment-support)
8. [Cost Considerations](#cost-considerations)
9. [Future Extensibility](#future-extensibility)

---

## High-Level Overview

The **Event-Driven Sleep Audio Pipeline** is a serverless AWS-native system that accepts raw audio uploads from users (voice recordings, ambient sounds, guided meditations) and transforms them into polished sleep audio artifacts. The pipeline is fully event-driven: no polling, no always-on compute, and no manual orchestration.

The pipeline proceeds through four logical phases:

| Phase | Description |
|---|---|
| **Ingestion** | User uploads a raw audio file to the S3 input bucket. |
| **Orchestration** | EventBridge detects the upload and triggers a Step Functions state machine. |
| **Processing** | The state machine coordinates validation, AI enhancement (Polly / Bedrock), and metadata extraction. |
| **Delivery** | Processed audio is saved to the S3 output bucket; metadata lands in DynamoDB; SNS emits a completion or failure notification. |

All infrastructure is defined as AWS CDK constructs in Go, deployed across `dev`, `stage`, and `prod` environments through CDK context values.

---

## Data Flow

### Ingestion

1. A client (mobile app, web frontend, or direct SDK call) uploads a raw audio file to the **S3 Input Bucket** under a key pattern such as `uploads/{user_id}/{timestamp}/{filename}`.
2. S3 emits an `Object Created` event to **Amazon EventBridge** when EventBridge notifications are enabled on the bucket.
### Orchestration

3. An **EventBridge Rule** matches the `aws.s3` source and `Object Created` detail type, filtering by the `uploads/` key prefix. On match, it invokes an **AWS Step Functions** Express Workflow as the target.

### Processing

The Step Functions state machine executes the following states in sequence:

4. **Validate Input** - A Lambda task reads the object metadata from S3 and confirms the file type (e.g., `.mp3`, `.wav`, `.m4a`) and size are within acceptable bounds. On failure, it transitions directly to the Error Notification state.
5. **Extract Metadata** - A Lambda task calls the S3 `HeadObject` API to extract duration, format, bit rate, and user ID from the S3 key. It writes an initial record to **DynamoDB** with `processing_status = IN_PROGRESS`.
6. **AI Enhancement (Choice)** - A Choice state inspects the job configuration embedded in the EventBridge event payload:
   - If `enhancement_type = polly`, invoke **Amazon Polly** to synthesise a soothing narration layer or convert a text script to speech.
   - If `enhancement_type = bedrock`, invoke **Amazon Bedrock** (e.g., `amazon.titan-tts-v1` or a compatible audio model) to generate ambient sleep sounds or blend audio layers using AI.
   - If `enhancement_type = passthrough`, skip AI processing and proceed directly to packaging.
7. **Package Output** - A Lambda task merges the original audio with any AI-generated layer, applies normalisation, and writes the final artifact to the **S3 Output Bucket** under `processed/{user_id}/{job_id}/{filename}`.
8. **Update Metadata** - A Lambda task updates the DynamoDB record with `processing_status = COMPLETED`, the output S3 key, duration, and a processing timestamp.

### Delivery

9. On successful completion, the state machine publishes a **success notification** to an **SNS Topic** (e.g., `SleepAudioProcessed`).
10. On any unhandled error, the state machine's Catch block publishes a **failure notification** to the same or a separate **SNS Error Topic** and updates DynamoDB with `processing_status = FAILED`.
11. Downstream consumers (push notification services, analytics, data lakes) subscribe to the SNS topic via SQS fan-out or direct Lambda triggers.

---

## System Diagram

```mermaid
flowchart TD
    Client([Client / Mobile App])

    subgraph Ingestion
        S3In["S3 Input Bucket\n(uploads/{user_id}/...)"]
    end

    subgraph Orchestration
        EB["EventBridge\n(aws.s3 ObjectCreated rule)"]
        SFN["Step Functions\nExpress Workflow"]
    end

    subgraph Processing
        LambdaValidate["Lambda: Validate Input"]
        LambdaMeta["Lambda: Extract Metadata"]
        Choice{Enhancement\nType?}
        Polly["Amazon Polly\n(TTS / narration)"]
        Bedrock["Amazon Bedrock\n(ambient AI audio)"]
        Passthrough["Passthrough\n(no AI)"]
        LambdaPackage["Lambda: Package Output"]
        LambdaUpdate["Lambda: Update Metadata"]
    end

    subgraph Storage
        S3Out["S3 Output Bucket\n(versioning enabled)"]
        DDB["DynamoDB\n(audio_jobs table)"]
    end

    subgraph Notifications
        SNSOk["SNS: SleepAudioProcessed\n(success)"]
        SNSErr["SNS: SleepAudioError\n(failure)"]
    end

    subgraph Observability
        CWLogs["CloudWatch Logs"]
        CWAlarms["CloudWatch Alarms"]
    end

    Client -->|upload| S3In
    S3In -->|Object Created event| EB
    EB -->|trigger| SFN

    SFN --> LambdaValidate
    LambdaValidate -->|valid| LambdaMeta
    LambdaValidate -->|invalid| SNSErr

    LambdaMeta -->|write IN_PROGRESS| DDB
    LambdaMeta --> Choice

    Choice -->|polly| Polly
    Choice -->|bedrock| Bedrock
    Choice -->|passthrough| Passthrough

    Polly --> LambdaPackage
    Bedrock --> LambdaPackage
    Passthrough --> LambdaPackage

    LambdaPackage -->|write artifact| S3Out
    LambdaPackage --> LambdaUpdate
    LambdaUpdate -->|update COMPLETED| DDB
    LambdaUpdate --> SNSOk

    SFN -->|Catch all errors| SNSErr
    SNSErr -->|update FAILED| DDB

    SFN -. logs .-> CWLogs
    LambdaValidate -. logs .-> CWLogs
    LambdaMeta -. logs .-> CWLogs
    LambdaPackage -. logs .-> CWLogs
    LambdaUpdate -. logs .-> CWLogs
    CWLogs -. metric filters .-> CWAlarms
```

---

## AWS Services and Rationale

| Service | Role in Pipeline | Why This Service |
|---|---|---|
| **S3 (Input Bucket)** | Receives raw audio uploads from clients | Infinitely scalable object storage; native EventBridge integration; supports presigned URLs for secure client uploads |
| **S3 (Output Bucket)** | Stores processed audio artifacts with versioning | Versioning preserves reprocessing history; lifecycle policies manage cost; same SDK ergonomics as input bucket |
| **EventBridge** | Routes S3 upload events to Step Functions | Decouples ingestion from processing; filter rules avoid triggering on unrelated S3 activity; native retry and dead-letter support |
| **Step Functions (Express)** | Orchestrates the multi-step processing workflow | Express Workflows suit high-throughput, short-duration jobs; built-in retry/catch logic; visual debugging in the AWS console; avoids Lambda chaining complexity |
| **Lambda** | Executes individual processing tasks | Pay-per-invocation; auto-scales with upload volume; easy to test and deploy independently per state |
| **Amazon Polly** | Text-to-speech narration generation | Managed TTS with multiple neural voices; no ML expertise required; SSML support for pacing and pauses |
| **Amazon Bedrock** | AI-generated ambient audio and enhancement | Access to foundation models via a single API; no GPU infrastructure to manage; model choice remains flexible |
| **DynamoDB** | Job metadata and processing status | Single-digit millisecond latency; serverless scaling; straightforward key design for `user_id + job_id` lookups |
| **SNS** | Completion and error notifications | Fan-out to multiple downstream consumers (SQS queues, Lambda, email, push); fully managed; decouples producers from consumers |
| **CloudWatch Logs** | Centralised log aggregation | Native integration with Lambda and Step Functions; metric filters enable alarm creation without extra infrastructure |
| **CloudWatch Alarms** | Alerting on error rates and latency | Low operational overhead; integrates with SNS for PagerDuty / Slack routing |
| **IAM** | Least-privilege access between services | Fine-grained resource policies; service-linked roles avoid credential management |
| **KMS** | Encryption at rest for S3 and DynamoDB | Centralised key management; audit trail via CloudTrail; supports per-environment key rotation policies |

---

## Security Design

### Least-Privilege IAM

Each Lambda function and Step Functions state machine is assigned a **dedicated IAM role** scoped to the minimum set of actions it requires:

- The Validate Lambda may only call `s3:GetObject` and `s3:HeadObject` on the input bucket.
- The Package Lambda may only call `s3:GetObject` on the input bucket and `s3:PutObject` on the output bucket.
- The Update Lambda may only call `dynamodb:PutItem` and `dynamodb:UpdateItem` on the `audio_jobs` table.
- Step Functions may only invoke the specific Lambda ARNs in its state machine definition.
- EventBridge may only start executions on the specific Step Functions state machine ARN.

### Encryption at Rest

- Both S3 buckets use **SSE-KMS** with per-environment KMS Customer Managed Keys (CMKs).
- DynamoDB uses **AWS-managed encryption** (SSE enabled by default) with the option to upgrade to CMKs in production.
- Lambda environment variables containing configuration secrets are encrypted with KMS.

### Private Buckets

- Both S3 buckets have **Block Public Access** enabled on all four settings (`BlockPublicAcls`, `IgnorePublicAcls`, `BlockPublicPolicy`, `RestrictPublicBuckets`).
- Clients upload using **presigned URLs** generated server-side, valid for a short TTL (e.g., 15 minutes). No long-lived credentials are distributed to clients.

### Network Isolation

- Lambda functions run in a **VPC** (optional, configurable via CDK context) with no direct internet access. Outbound calls to AWS APIs use VPC Interface Endpoints.
- S3 and DynamoDB are accessed via **VPC Gateway Endpoints**, ensuring traffic does not traverse the public internet.

---

## Observability

### CloudWatch Logs

All Lambda functions and the Step Functions state machine are configured to emit structured JSON logs to dedicated CloudWatch Log Groups:

- `/aws/lambda/sleep-audio-validate-{env}`
- `/aws/lambda/sleep-audio-metadata-{env}`
- `/aws/lambda/sleep-audio-package-{env}`
- `/aws/lambda/sleep-audio-update-{env}`
- `/aws/states/sleep-audio-pipeline-{env}`

Log retention is set per environment (e.g., 7 days in `dev`, 90 days in `prod`).

### CloudWatch Alarms

| Alarm | Metric | Threshold | Action |
|---|---|---|---|
| Lambda error rate | `Errors / Invocations` | > 5% over 5 min | Notify SNS Error Topic |
| Step Functions execution failures | `ExecutionsFailed` | >= 1 in 1 min | Notify SNS Error Topic |
| Step Functions throttles | `ExecutionThrottled` | >= 1 in 1 min | Notify SNS Error Topic |
| DynamoDB throttles | `UserErrors` | >= 5 in 5 min | Notify SNS Error Topic |
| S3 4xx errors on input bucket | `4xxErrors` | >= 10 in 5 min | Notify SNS Error Topic |

### Distributed Tracing

AWS X-Ray active tracing is enabled on all Lambda functions and the Step Functions state machine, allowing end-to-end latency analysis across the full pipeline.

---

## Multi-Environment Support

The CDK app uses **context variables** to differentiate behaviour between `dev`, `stage`, and `prod`:

```json
{
  "env": "dev",
  "logRetentionDays": 7,
  "enableVpc": false,
  "bedrockEnabled": false,
  "alarmActions": []
}
```

Context is supplied at synth time:

```bash
cdk synth -c env=prod -c enableVpc=true -c bedrockEnabled=true
```

Stack names follow the pattern `SleepAudioPipeline-{env}`, ensuring separate CloudFormation stacks per environment with no resource name collisions.

---

## Cost Considerations

- **Step Functions Express Workflows** are billed per state transition and duration, making them very cost-effective for short audio jobs (typically under 60 seconds end-to-end).
- **Lambda** is billed per invocation and GB-second. Arm64 (Graviton) runtime should be used to reduce cost by ~20% at equivalent performance.
- **S3** input objects should have a lifecycle policy to transition to Glacier after 30 days (dev/stage) or retain indefinitely (prod).
- **DynamoDB** on-demand capacity is recommended for variable workloads; switch to provisioned capacity with auto-scaling once traffic patterns are known.
- **Polly** and **Bedrock** are the dominant variable cost drivers. Request batching and caching of Polly output for identical text inputs should be considered at scale.
- **CloudWatch Logs** ingestion costs can be managed by filtering out DEBUG-level logs in `stage` and `prod`.

---

## Future Extensibility

| Capability | Approach |
|---|---|
| **Waveform visualisation** | Add a new Step Functions state that calls a Lambda to generate a waveform image via FFmpeg layer and store it alongside the audio in S3. |
| **User-defined playlists** | Add an AppSync API backed by DynamoDB to allow users to sequence processed tracks. |
| **Real-time streaming** | Replace batch S3 upload with Kinesis Data Streams ingestion; Step Functions workflow triggers on stream events. |
| **Multi-region failover** | Enable S3 Cross-Region Replication and DynamoDB Global Tables; Route 53 health checks control API routing. |
| **Content moderation** | Insert an Amazon Rekognition (audio transcription) + Comprehend moderation step before the AI enhancement state. |
| **Cost attribution** | Tag all resources with `user_id` via S3 object tags propagated through the workflow; use AWS Cost Explorer tag-based allocation. |
| **Async client polling** | Replace synchronous presigned-URL upload with an API Gateway + WebSocket endpoint that pushes job status updates to the client in real time. |
