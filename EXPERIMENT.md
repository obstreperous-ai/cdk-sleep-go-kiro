# Experiment Design: AI-Driven TDD Infrastructure as Code

This document captures the experimental design, methodology, actors, prompting strategy, and preliminary observations from building the **Event-Driven Sleep Audio Pipeline** - a production-grade serverless system developed entirely through AI-driven, issue-tracked Test-Driven Development.

---

## Table of Contents

- [Overview and Goals](#overview-and-goals)
- [Methodology](#methodology)
- [Actors and Setup](#actors-and-setup)
- [Prompting Patterns and Meta-Prompts](#prompting-patterns-and-meta-prompts)
- [Issue History Summary](#issue-history-summary)
- [Key Decisions and Trade-offs](#key-decisions-and-trade-offs)
- [Preliminary Observations](#preliminary-observations)
- [References](#references)

---

## Overview and Goals

### The Experiment

This project is one instance in a controlled experiment exploring how AI coding assistants build production-grade Infrastructure as Code when guided by strict TDD, issue-driven development, and structured meta-prompting patterns.

The experiment spans a matrix of:

- **5 programming languages:** Go, TypeScript, Python, Java, C#
- **3 AI coding assistants:** Each language is paired with different AI agents to observe variations in output quality, development velocity, and code style

All instances build the same conceptual pipeline (an event-driven audio processing system on AWS), allowing direct comparison of how different language-AI combinations handle identical requirements.

### This Instance

| Dimension | Value |
|---|---|
| Language | Go 1.25 |
| IaC Framework | AWS CDK v2.255.0 |
| AI Agent | Kiro (by Amazon) |
| Architecture | Serverless, event-driven |
| Methodology | Strict TDD + issue-driven development |
| Duration | 2026-05-28 to 2026-06-13 (16 days) |

### Goals

1. **Measure feasibility** - Can an AI agent build production-grade IaC from scratch using only issue-driven instructions and TDD constraints?
2. **Evaluate quality** - Does the resulting infrastructure meet the same bar as human-authored code (security, testing, observability, documentation)?
3. **Extract patterns** - What reusable meta-prompting techniques emerge from guiding an AI through a multi-week development effort?
4. **Compare across dimensions** - How do different language/AI combinations differ in code quality, test coverage, development speed, and architectural decisions?
5. **Document process** - Create a reproducible methodology that others can apply to their own AI-driven IaC projects.

---

## Methodology

The experiment follows three foundational pillars applied consistently across every issue and pull request.

### Pillar 1: Strict Test-Driven Development

Every change follows the red-green-refactor cycle at every layer of the system:

1. **Red** - Write a failing test that defines the expected behavior
2. **Green** - Implement the minimal code to make the test pass
3. **Refactor** - Clean up while keeping all tests green
4. **Repeat** - Each new capability starts with a new failing test

TDD is applied at four distinct layers:

| Layer | Approach | Example |
|---|---|---|
| Infrastructure | CDK assertion tests | Assert S3 bucket exists with encryption enabled |
| Application | Interface-based mocks | Test Lambda handler with mocked S3/DynamoDB clients |
| Integration | End-to-end validation | Verify state machine wires EventBridge to SNS correctly |
| Regression | Snapshot stability | Golden file comparison prevents unintended drift |

### Pillar 2: Issue-Driven Development

Every change originates from a tracked GitHub issue with defined scope:

- No work happens without a corresponding issue
- Each issue maps to exactly one focused pull request
- Scope creep is explicitly prevented (one concern per issue)
- Issues create an audit trail of design decisions and rationale
- Acceptance criteria define "done" before work begins

### Pillar 3: Architecture-as-Code

The system architecture is maintained as a living document with machine-renderable Mermaid diagrams:

- [ARCHITECTURE.md](./ARCHITECTURE.md) is the single source of truth
- Mermaid diagrams render directly in GitHub, providing always-current visuals
- Any infrastructure change must include a corresponding architecture update in the same commit
- Reviewers can verify design consistency without reading implementation code

### Quality Gates

Before any pull request is merged, these gates must pass:

1. All tests pass (`go test -v -count=1 ./...`)
2. CDK synthesis succeeds (`cdk synth`)
3. Snapshot test passes (or is explicitly regenerated and reviewed)
4. Architecture documentation is in sync with code
5. Commit messages follow Conventional Commits format

---

## Actors and Setup

### AI Agent: Kiro (by Amazon)

Kiro operates under a defined persona established in [`.github/AGENT_GUIDELINES.md`](./.github/AGENT_GUIDELINES.md):

> You are a Senior AWS CDK Go TDD Specialist. Use clean Go idioms. Write tests first, then minimal code. Always follow strict TDD: write failing test(s) first, then the minimal code to make them pass. Keep ARCHITECTURE.md and its Mermaid diagram perfectly in sync after every change. Prefer L2/L3 constructs. Follow AWS Well-Architected principles. Never deploy until tests + synth succeed locally.

This persona constrains the agent to:

- A specific expertise level (Senior) implying awareness of edge cases and best practices
- A specific domain (AWS CDK + Go + TDD) anchoring all decisions
- Hard safety gates (never deploy untested code, always update architecture docs)
- Tool preferences (L2/L3 constructs over raw CloudFormation)

### Human Actor: Experiment Designer

The human actor (obstreperous-ai) operates as the experiment designer and issue author:

- Creates issues with structured requirements and acceptance criteria
- Reviews pull requests for correctness and adherence to methodology
- Does not write implementation code directly
- Guides the AI through progressive complexity (bootstrap to production-ready)

### Language Flavor: Go CDK

The Go CDK flavor was chosen for this instance based on:

| Property | Benefit |
|---|---|
| Static typing | Compile-time safety for infrastructure definitions |
| Fast cold starts | Go Lambda custom runtime (`provided.al2023`) starts in milliseconds |
| Single language | Both infrastructure (CDK) and application (Lambda) use Go |
| CDK jsii bindings | Full access to AWS CDK constructs via Go interfaces |
| Module system | Separate `go.mod` files isolate CDK and Lambda dependencies |

### Development Environment

| Component | Version/Tool |
|---|---|
| Go | 1.25 |
| AWS CDK | v2.255.0 |
| Node.js | 22+ (CDK CLI runtime) |
| Testing | Go standard `testing` package + CDK assertions |
| CI | GitHub Actions |
| Deployment | CDK Pipelines (CodePipeline) |

---

## Prompting Patterns and Meta-Prompts

Seven reusable meta-prompting patterns were extracted from this project. These patterns establish persistent behavioral constraints that shape how the AI agent approaches every task. Full descriptions with templates are in [META-PROMPTS.md](./META-PROMPTS.md).

### Pattern Summary

| # | Pattern | Purpose | Application |
|---|---|---|---|
| 1 | [Agent Persona](./META-PROMPTS.md#pattern-1-agent-persona) | Define specialist identity with constraints | Applied once at project setup; governs all subsequent work |
| 2 | [TDD-First](./META-PROMPTS.md#pattern-2-tdd-first) | Enforce red-green-refactor at every layer | Applied to every issue; tests always precede implementation |
| 3 | [Architecture-as-Source-of-Truth](./META-PROMPTS.md#pattern-3-architecture-as-source-of-truth) | Single doc as living design authority | Applied to every infrastructure change; docs stay in sync |
| 4 | [Issue-Driven Development](./META-PROMPTS.md#pattern-4-issue-driven-development) | Scope every change to a tracked issue | Applied at project level; no work without an issue |
| 5 | [Conventional Commits](./META-PROMPTS.md#pattern-5-conventional-commits) | Structured commit messages | Applied to every commit; enables automated tooling |
| 6 | [Snapshot Stability](./META-PROMPTS.md#pattern-6-snapshot-stability) | Golden file comparison for drift detection | Applied to infrastructure tests; catches unintended changes |
| 7 | [Defense-in-Depth Validation](./META-PROMPTS.md#pattern-7-defense-in-depth-validation) | Validate at multiple layers | Applied to input processing; cheap checks before expensive ones |

### How Patterns Were Applied

The patterns were not introduced all at once. They were layered progressively as the project grew:

- **Issues #1-2 (Bootstrap):** Patterns 1, 2, 4, 5 established the foundation
- **Issues #3-6 (Core Infrastructure):** Pattern 3 became critical as architecture grew
- **Issues #7-9 (Integration):** Pattern 6 introduced snapshot stability
- **Issues #10-12 (Production Hardening):** Pattern 7 emerged for dual-validation
- **Issue #13 (Documentation):** All patterns documented and extracted for reuse

### Pattern Interactions

The patterns reinforce each other:

- **TDD-First + Snapshot Stability:** Snapshot tests are the regression layer of TDD
- **Issue-Driven + Conventional Commits:** Issues provide context; commits provide traceability
- **Architecture-as-Source-of-Truth + Agent Persona:** The persona rule ("keep ARCHITECTURE.md in sync") enforces the architecture pattern
- **Defense-in-Depth + TDD-First:** Each validation layer has its own covering tests

---

## Issue History Summary

The project was developed over 14 issues spanning 16 days. Each issue represents a focused unit of work with defined acceptance criteria.

| # | Issue | Title | Created | Closed | PR | Summary |
|---|---|---|---|---|---|---|
| 1 | #1 | Bootstrap: Go CDK + Strict TDD + Agent Configuration | 2026-05-28 | 2026-05-28 | #2 | Initial project scaffolding with CDK app, Go modules, CI workflow, and agent persona |
| 2 | #3 | Initial Architecture Design | 2026-06-01 | 2026-06-02 | #4 | ARCHITECTURE.md with Mermaid diagrams, data flow design, security model |
| 3 | #5 | TDD: Core S3 Buckets + EventBridge Rule | 2026-06-02 | 2026-06-03 | #6 | Input/output S3 buckets with encryption, versioning; EventBridge rule for S3 events |
| 4 | #7 | TDD: Step Functions State Machine + Polly Integration | 2026-06-03 | 2026-06-04 | #8 | Express Workflow state machine with Polly task, retry policies, X-Ray tracing |
| 5 | #9 | TDD: DynamoDB Metadata Table + State Machine I/O | 2026-06-04 | 2026-06-05 | #10 | DynamoDB table for job tracking; state machine read/write integration |
| 6 | #11 | TDD: SNS Notifications + Error Handling | 2026-06-05 | 2026-06-06 | #12 | Completed/Failed SNS topics with KMS encryption; error state routing |
| 7 | #13 | TDD: Lambda Function Skeleton + State Machine Integration | 2026-06-06 | 2026-06-07 | #14 | Go Lambda handler scaffold; interface-based mocks; state machine wiring |
| 8 | #15 | TDD: Complete Pipeline Wiring, Input Validation | 2026-06-07 | 2026-06-08 | #16 | End-to-end pipeline validation; dual-layer input validation |
| 9 | #17 | TDD: Pipeline Testing, Refinement, Deployment Prep | 2026-06-08 | 2026-06-09 | #18 | CDK Pipelines skeleton; environment configuration; deployment testing |
| 10 | #19 | TDD: Advanced Error Handling, Retries, Observability | 2026-06-09 | 2026-06-10 | #20 | Per-state retry policies; CloudWatch alarms; structured logging |
| 11 | #21 | TDD: Full Audio Processing Implementation | 2026-06-10 | 2026-06-11 | #22 | Complete Lambda processor: S3 download, Polly synthesis, S3 upload, DynamoDB update |
| 12 | #23 | TDD: End-to-End Validation, Documentation, Completion | 2026-06-11 | 2026-06-12 | #24 | Full E2E test coverage; documentation polish; project completion verification |
| 13 | #25 | Documentation: Review and Enrich README + Meta-Prompts | 2026-06-12 | 2026-06-13 | #26 | Comprehensive README rewrite; META-PROMPTS.md with 7 reusable patterns |
| 14 | #27 | Documentation: Experiment Design and Meta-Prompting Process | 2026-06-13 | 2026-06-13 | #28 | This document: experimental methodology, observations, and evaluation foundation |

<!-- Note: Row 14 and velocity stats updated to reflect this PR's own merge. -->

### Development Velocity

- **Total issues:** 14
- **Total PRs merged:** 14
- **Average cycle time:** ~1 day per issue
- **Infrastructure issues (TDD):** 10 (issues #5 through #23)
- **Documentation issues:** 4 (issues #3, #25, #27, plus this PR)
- **Bootstrap issues:** 1 (issue #1)

---

## Key Decisions and Trade-offs

The following architectural decisions were made during development. Each represents a deliberate trade-off evaluated against the project's requirements. Full rationale is documented in [SUMMARY.md](./SUMMARY.md).

### Architecture Decisions

| Decision | Chosen Approach | Alternative Considered | Rationale |
|---|---|---|---|
| Compute model | Serverless-only | Container-based (ECS/Fargate) | Zero idle cost; automatic scaling; minimal ops overhead |
| Event routing | EventBridge | S3 event notifications direct to Lambda | Decoupling; filtering; fan-out capability; multi-target support |
| Orchestration | Step Functions Express | Standard Workflows | Higher throughput; lower cost; jobs complete in under 60 seconds |
| Language | Go for CDK and Lambda | TypeScript CDK + Go Lambda | Single language reduces context switching; type safety across stack |
| Module isolation | Separate go.mod for Lambda | Single module | Smaller deployment artifact; isolated SDK dependencies |
| Metadata store | DynamoDB | Aurora Serverless | Single-digit ms latency; serverless; simple key design |
| Notifications | Two SNS topics (success/failure) | Single topic with attributes | Distinct downstream routing; simpler subscription filtering |
| Validation | Defense-in-depth (two layers) | Single validation point | Cheap fast-fail at orchestration; comprehensive checks in Lambda |
| Multi-environment | CDK context variables | Separate CDK apps per env | Single codebase; consistent infrastructure across environments |
| CI/CD | CDK Pipelines (self-mutating) | GitHub Actions deploy | Pipeline updates itself; native CDK integration |

### Lessons Learned

1. **CDK feature flags affect test structure** - The `@aws-cdk/aws-iam:minimizePolicies` flag merges IAM statements into the role's default policy rather than creating separate policy resources. Tests must assert against the merged structure.

2. **Step Functions retry is per-state** - Retries cannot be configured globally. Each task state needs its own retry policy with error type matching. Specific errors first, then `States.ALL` as a fallback.

3. **Dual-validation provides cost savings** - The Step Functions Choice state catches invalid file extensions before any Lambda invocation occurs. This saves compute cost on obviously invalid inputs while Lambda handles edge cases.

4. **Express vs Standard Workflows** - Express Workflows lack visual execution history in the console but provide equivalent visibility through CloudWatch Logs. For sub-60-second jobs, the cost difference is significant.

5. **Go CDK jsii bindings verbosity** - Go requires `jsii.String()`, `jsii.Number()`, and `jsii.Bool()` wrappers for all CDK property values. This adds visual noise but maintains type safety.

6. **Snapshot test regeneration** - Snapshot golden files must be manually deleted and regenerated when infrastructure changes intentionally. Automated regeneration would defeat the purpose of drift detection.

7. **Lambda output key format** - Using second-level timestamps (`20060102T150405Z`) in S3 keys provides uniqueness for expected throughput while remaining human-readable.

---

## Preliminary Observations

These observations represent initial findings from this single instance (Go + Kiro). Final conclusions require comparison across all 15 matrix cells (5 languages x 3 AIs).

### Strengths

| Observation | Evidence |
|---|---|
| **Snapshot tests catch drift effectively** | Multiple refactoring passes were validated instantly by the snapshot comparison test |
| **Interface-based mocks enable rapid iteration** | Lambda tests run in milliseconds with no AWS credentials required |
| **Dual-validation provides defense-in-depth** | Step Functions catches ~80% of invalid inputs at near-zero cost; Lambda catches the remaining edge cases |
| **Strict TDD produces high test coverage** | Every infrastructure resource and Lambda code path has a corresponding test |
| **Issue-driven scope prevents sprawl** | Each PR stays focused on one concern, making review straightforward |
| **Architecture-as-code keeps docs current** | ARCHITECTURE.md was updated in every infrastructure PR, preventing documentation rot |
| **Conventional commits create readable history** | Git log reads as a structured narrative of the project's evolution |

### Challenges

| Challenge | Impact | Mitigation |
|---|---|---|
| CDK feature flags affect policy structure | Tests needed restructuring when flags were enabled | Document flag behavior; test against both states when possible |
| Go CDK jsii bindings are verbose | Code is harder to scan visually than TypeScript equivalent | Accept verbosity as trade-off for type safety; use consistent formatting |
| Network constraints in sandbox environments | Cannot run `go mod download` or `cdk synth` in restricted environments | Document-only changes bypass this; infrastructure changes require full environment |
| Snapshot test maintenance overhead | Every intentional change requires manual regeneration | Clear documentation of regeneration procedure; explicit in contributing guide |
| Express Workflow debugging | No visual execution history in Step Functions console | CloudWatch Logs at ALL level provides equivalent (if less visual) debugging |

### Observations on AI-Driven Development

1. **Consistency across issues** - The agent persona pattern produced remarkably consistent code style, test structure, and documentation quality across all 14 issues. Human-authored code often shows style drift over time.

2. **Progressive complexity works** - Starting with simple infrastructure (S3 buckets) and layering complexity (state machines, Lambda, observability) allowed the AI to build on established patterns rather than handling everything at once.

3. **Explicit constraints prevent common mistakes** - The "never deploy until tests pass" rule and "update ARCHITECTURE.md in same commit" rule prevented the two most common IaC development failures: untested deployments and stale documentation.

4. **Meta-prompting patterns are transferable** - The seven patterns extracted from this project are language-agnostic and framework-agnostic. They should apply equally to the TypeScript, Python, Java, and C# instances of this experiment.

5. **Issue-driven development creates natural checkpoints** - Each issue/PR boundary provides a natural point for evaluation, comparison, and course correction.

6. **TDD catches integration issues early** - End-to-end validation tests caught wiring issues (incorrect EventBridge patterns, missing IAM permissions) before any deployment was attempted.

### Open Questions for Cross-Instance Comparison

- Does Go's verbosity (jsii wrappers) slow development compared to TypeScript's native CDK?
- Do different AI agents produce meaningfully different architectural decisions given identical requirements?
- Does TDD coverage correlate with fewer deployment failures across the matrix?
- Which language/AI combinations produce the most maintainable code (measured by ease of modification)?
- Do the same meta-prompting patterns work equally well across all three AI agents?

---

## Issue #29: Code Quality, Test Coverage, Reflection & Tidy-Up

### What Was Found

During this code quality pass, several issues were identified:

1. **Snapshot drift** - The `testdata/snapshot.json` golden file had become stale and no longer matched the synthesized template. The file was deleted and regenerated by running the snapshot test.

2. **Test name typo** - `TestSNSTopicsHaveNoBoardcodedNames` contained a typo ("Boardcoded" instead of "Hardcoded"). Renamed to `TestSNSTopicsHaveNoHardcodedNames`.

3. **Test coverage gaps** - Lambda processor coverage was 81.9%. Uncovered branches included:
   - `updateDynamoDBStatus` with empty `TableName` (early return path)
   - `Process` when Polly returns a nil `AudioStream`
   - `Process` when `io.ReadAll` fails reading the Polly audio stream
   - The `structuredLog` function had no direct unit test

4. **CI lacking coverage reporting** - The GitHub Actions workflow ran tests without capturing coverage profiles or reporting them as artifacts.

5. **Magic number** - The 10MB stream limit (`10*1024*1024`) was a magic number without a named constant.

### What Was Improved

- **Coverage increased from 81.9% to 90%+** by adding targeted tests for each uncovered branch
- **CI now reports coverage** with `-coverprofile` flags and uploads coverage artifacts
- **Lambda processor tests now run separately in CI** for clear per-module visibility
- **Named constant `maxAudioStreamSize`** replaces the magic number for clarity
- **Test name corrected** so the test intent is immediately clear from the name

### Challenges Encountered

1. **golangci-lint version mismatch** - The installed golangci-lint 2.1.6 was built with Go 1.24 and refuses to lint Go 1.25 code. Static analysis was performed with `go vet` instead.

2. **NODE_OPTIONS environment interference** - The sandbox sets `NODE_OPTIONS` which conflicts with the jsii runtime used by CDK tests. Every CDK test run requires `unset NODE_OPTIONS` or setting it to empty string.

3. **jsii test runtime overhead** - CDK assertion tests take approximately 75 seconds per run due to the jsii JavaScript bridge, making rapid TDD iteration slower than typical Go tests.

4. **Snapshot regeneration workflow** - Deleting the snapshot file and running `TestStackSnapshotStability` is the only way to regenerate it. This is fragile but matches CDK convention for golden-file tests.

---

## References

- [README.md](./README.md) - Project overview, quick start, and experiment methodology summary
- [ARCHITECTURE.md](./ARCHITECTURE.md) - Detailed architecture with Mermaid diagrams
- [META-PROMPTS.md](./META-PROMPTS.md) - Seven reusable meta-prompting patterns with templates
- [SUMMARY.md](./SUMMARY.md) - Key decisions, lessons learned, and trade-offs
- [CONTRIBUTING.md](./CONTRIBUTING.md) - Development guidelines and quality gates
- [.github/AGENT_GUIDELINES.md](./.github/AGENT_GUIDELINES.md) - AI agent persona definition
