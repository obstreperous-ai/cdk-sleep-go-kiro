package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatchactions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsstepfunctionstasks"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type CdkBaseStackProps struct {
	awscdk.StackProps
}

func NewCdkBaseStack(scope constructs.Construct, id string, props *CdkBaseStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// S3 Input Bucket - receives raw audio uploads
	inputBucket := awss3.NewBucket(stack, jsii.String("SleepAudioInputBucket"), &awss3.BucketProps{
		Encryption:         awss3.BucketEncryption_S3_MANAGED,
		Versioned:          jsii.Bool(true),
		BlockPublicAccess:  awss3.BlockPublicAccess_BLOCK_ALL(),
		EventBridgeEnabled: jsii.Bool(true),
		RemovalPolicy:      awscdk.RemovalPolicy_DESTROY,
	})

	// S3 Output Bucket - stores processed audio artifacts
	outputBucket := awss3.NewBucket(stack, jsii.String("SleepAudioOutputBucket"), &awss3.BucketProps{
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
		Versioned:         jsii.Bool(true),
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
	})

	// CloudWatch Log Group for state machine logging
	logGroup := awslogs.NewLogGroup(stack, jsii.String("SleepAudioPipelineLogGroup"), &awslogs.LogGroupProps{
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	// DynamoDB Table for audio pipeline metadata
	metadataTable := awsdynamodb.NewTable(stack, jsii.String("SleepAudioMetadataTable"), &awsdynamodb.TableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("audioId"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		BillingMode:         awsdynamodb.BillingMode_PAY_PER_REQUEST,
		Encryption:          awsdynamodb.TableEncryption_AWS_MANAGED,
		PointInTimeRecoverySpecification: &awsdynamodb.PointInTimeRecoverySpecification{
			PointInTimeRecoveryEnabled: jsii.Bool(true),
		},
		RemovalPolicy:       awscdk.RemovalPolicy_DESTROY,
	})

	// SNS Topic for pipeline completion notifications (encrypted with AWS-managed SNS KMS key)
	completedTopic := awssns.NewTopic(stack, jsii.String("SleepAudioPipelineCompleted"), &awssns.TopicProps{
		MasterKey: awskms.Alias_FromAliasName(stack, jsii.String("SnsKmsKeyCompleted"), jsii.String("alias/aws/sns")),
	})

	// SNS Topic for pipeline failure notifications (encrypted with AWS-managed SNS KMS key)
	failedTopic := awssns.NewTopic(stack, jsii.String("SleepAudioPipelineFailed"), &awssns.TopicProps{
		MasterKey: awskms.Alias_FromAliasName(stack, jsii.String("SnsKmsKeyFailed"), jsii.String("alias/aws/sns")),
	})

	// Lambda Function - SleepAudioProcessor
	// Placeholder for audio processing/validation logic. Uses Go custom runtime.
	processorLambda := awslambda.NewFunction(stack, jsii.String("SleepAudioProcessor"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2023(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("lambda/processor"), nil),
		Tracing: awslambda.Tracing_ACTIVE,
		Environment: &map[string]*string{
			"TABLE_NAME":         metadataTable.TableName(),
			"OUTPUT_BUCKET_NAME": outputBucket.BucketName(),
			"INPUT_BUCKET_NAME":  inputBucket.BucketName(),
		},
	})

	// Grant Lambda read/write access to the DynamoDB metadata table
	metadataTable.GrantReadWriteData(processorLambda)

	// Grant Lambda write access to the output bucket for storing processed audio
	outputBucket.GrantWrite(processorLambda, nil, nil)

	// Grant Lambda read access to the input bucket for downloading source audio
	inputBucket.GrantRead(processorLambda, nil)

	// Step Functions DynamoDB PutItem task - write initial PROCESSING record
	// ConditionExpression prevents overwriting in-flight records: only allows
	// the PutItem if no record exists for this audioId, or the existing record
	// has already reached a terminal state (COMPLETED or FAILED).
	writeInitialRecord := awsstepfunctionstasks.NewDynamoPutItem(stack, jsii.String("WriteInitialRecord"), &awsstepfunctionstasks.DynamoPutItemProps{
		Table: metadataTable,
		Item: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			"audioId":   awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key"))),
			"status":    awsstepfunctionstasks.DynamoAttributeValue_FromString(jsii.String("PROCESSING")),
			"bucket":    awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.bucket.name"))),
			"objectKey": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key"))),
			"createdAt": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.time"))),
		},
		ConditionExpression: jsii.String("attribute_not_exists(audioId) OR #s IN (:completed, :failed)"),
		ExpressionAttributeNames: &map[string]*string{
			"#s": jsii.String("status"),
		},
		ExpressionAttributeValues: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			":completed": awsstepfunctionstasks.DynamoAttributeValue_FromString(jsii.String("COMPLETED")),
			":failed":    awsstepfunctionstasks.DynamoAttributeValue_FromString(jsii.String("FAILED")),
		},
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// Step Functions CallAwsService task for Amazon Polly synthesizeSpeech
	pollyTask := awsstepfunctionstasks.NewCallAwsService(stack, jsii.String("PollyTask"), &awsstepfunctionstasks.CallAwsServiceProps{
		Service: jsii.String("polly"),
		Action:  jsii.String("synthesizeSpeech"),
		Parameters: &map[string]interface{}{
			"Text":         "Welcome to your sleep audio session. Relax and breathe deeply.",
			"OutputFormat": "mp3",
			"VoiceId":      "Joanna",
		},
		IamResources: &[]*string{jsii.String("*")},
		ResultPath:   jsii.String("$.pollyResult"),
	})

	// Step Functions LambdaInvoke task - process audio via SleepAudioProcessor Lambda
	// Payload extracts flat {audioId, bucket, objectKey} from the EventBridge envelope
	// RetryOnServiceExceptions disabled to avoid CDK's default retry overlapping with our custom retry policy
	processAudio := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("ProcessAudio"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: processorLambda,
		Payload: awsstepfunctions.TaskInput_FromObject(&map[string]interface{}{
			"audioId":   awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key")),
			"bucket":    awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.bucket.name")),
			"objectKey": awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key")),
		}),
		ResultPath:               jsii.String("$.processorResult"),
		RetryOnServiceExceptions: jsii.Bool(false),
	})

	// Step Functions DynamoDB UpdateItem task - mark as COMPLETED
	// Uses $$.State.EnteredTime for updatedAt to record actual completion time
	markCompleted := awsstepfunctionstasks.NewDynamoUpdateItem(stack, jsii.String("MarkCompleted"), &awsstepfunctionstasks.DynamoUpdateItemProps{
		Table: metadataTable,
		Key: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			"audioId": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key"))),
		},
		ExpressionAttributeValues: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			":status":    awsstepfunctionstasks.DynamoAttributeValue_FromString(jsii.String("COMPLETED")),
			":updatedAt": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$$.State.EnteredTime"))),
		},
		UpdateExpression: jsii.String("SET #s = :status, updatedAt = :updatedAt"),
		ExpressionAttributeNames: &map[string]*string{
			"#s": jsii.String("status"),
		},
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// Add retry for transient DynamoDB errors on MarkCompleted
	markCompleted.AddRetry(&awsstepfunctions.RetryProps{
		Errors:       &[]*string{awsstepfunctions.Errors_ALL()},
		Interval:     awscdk.Duration_Seconds(jsii.Number(2)),
		MaxAttempts:  jsii.Number(3),
		BackoffRate:  jsii.Number(2.0),
	})

	// Step Functions DynamoDB UpdateItem task - mark as FAILED (error handler)
	// Uses $$.State.EnteredTime for updatedAt to record actual failure time
	markFailed := awsstepfunctionstasks.NewDynamoUpdateItem(stack, jsii.String("MarkFailed"), &awsstepfunctionstasks.DynamoUpdateItemProps{
		Table: metadataTable,
		Key: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			"audioId": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key"))),
		},
		ExpressionAttributeValues: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			":status":    awsstepfunctionstasks.DynamoAttributeValue_FromString(jsii.String("FAILED")),
			":updatedAt": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$$.State.EnteredTime"))),
		},
		UpdateExpression: jsii.String("SET #s = :status, updatedAt = :updatedAt"),
		ExpressionAttributeNames: &map[string]*string{
			"#s": jsii.String("status"),
		},
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// Add retry for transient DynamoDB errors on MarkFailed
	markFailed.AddRetry(&awsstepfunctions.RetryProps{
		Errors:       &[]*string{awsstepfunctions.Errors_ALL()},
		Interval:     awscdk.Duration_Seconds(jsii.Number(2)),
		MaxAttempts:  jsii.Number(3),
		BackoffRate:  jsii.Number(2.0),
	})

	// SNS Publish task - notify on successful pipeline completion
	notifyCompleted := awsstepfunctionstasks.NewSnsPublish(stack, jsii.String("NotifyCompleted"), &awsstepfunctionstasks.SnsPublishProps{
		Topic: completedTopic,
		Message: awsstepfunctions.TaskInput_FromObject(&map[string]interface{}{
			"status":  "COMPLETED",
			"audioId": awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key")),
		}),
		Subject:    jsii.String("Sleep Audio Pipeline - Completed"),
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// Add retry for transient SNS errors on NotifyCompleted
	notifyCompleted.AddRetry(&awsstepfunctions.RetryProps{
		Errors:      &[]*string{awsstepfunctions.Errors_ALL()},
		Interval:    awscdk.Duration_Seconds(jsii.Number(2)),
		MaxAttempts: jsii.Number(3),
		BackoffRate: jsii.Number(2.0),
	})

	// Terminal state if completed notification fails after retries
	notifyCompletedFallback := awsstepfunctions.NewPass(stack, jsii.String("NotifyCompletedFallback"), &awsstepfunctions.PassProps{
		Result: awsstepfunctions.NewResult(jsii.String("Notification delivery failed but pipeline completed successfully")),
	})

	// Catch on NotifyCompleted so execution still succeeds if SNS fails
	notifyCompleted.AddCatch(notifyCompletedFallback, &awsstepfunctions.CatchProps{
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// SNS Publish task - notify on pipeline failure
	notifyFailed := awsstepfunctionstasks.NewSnsPublish(stack, jsii.String("NotifyFailed"), &awsstepfunctionstasks.SnsPublishProps{
		Topic: failedTopic,
		Message: awsstepfunctions.TaskInput_FromObject(&map[string]interface{}{
			"status":  "FAILED",
			"audioId": awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key")),
			"error":   awsstepfunctions.JsonPath_StringAt(jsii.String("$.error")),
		}),
		Subject:    jsii.String("Sleep Audio Pipeline - Failed"),
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// Add retry for transient SNS errors on NotifyFailed
	notifyFailed.AddRetry(&awsstepfunctions.RetryProps{
		Errors:      &[]*string{awsstepfunctions.Errors_ALL()},
		Interval:    awscdk.Duration_Seconds(jsii.Number(2)),
		MaxAttempts: jsii.Number(3),
		BackoffRate: jsii.Number(2.0),
	})

	// Terminal state if failed notification fails after retries
	notifyFailedFallback := awsstepfunctions.NewPass(stack, jsii.String("NotifyFailedFallback"), &awsstepfunctions.PassProps{
		Result: awsstepfunctions.NewResult(jsii.String("Notification delivery failed but pipeline failure was recorded")),
	})

	// Catch on NotifyFailed so execution still completes if SNS fails
	notifyFailed.AddCatch(notifyFailedFallback, &awsstepfunctions.CatchProps{
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// Wire failure path: MarkFailed -> NotifyFailed
	markFailed.Next(notifyFailed)

	// Add error handling: Polly task - specific error catches first, then generic fallback
	pollyTask.AddCatch(markFailed, &awsstepfunctions.CatchProps{
		Errors:     &[]*string{jsii.String("States.TaskFailed"), jsii.String("Polly.ServiceException")},
		ResultPath: jsii.String("$.error"),
	})
	pollyTask.AddCatch(markFailed, &awsstepfunctions.CatchProps{
		Errors:     &[]*string{jsii.String("States.ALL")},
		ResultPath: jsii.String("$.error"),
	})

	// Add retry for transient errors on PollyTask
	pollyTask.AddRetry(&awsstepfunctions.RetryProps{
		Errors:      &[]*string{jsii.String("States.TaskFailed"), jsii.String("Polly.ServiceException")},
		Interval:    awscdk.Duration_Seconds(jsii.Number(3)),
		MaxAttempts: jsii.Number(3),
		BackoffRate: jsii.Number(2.0),
	})

	// Add error handling: ProcessAudio Lambda - specific error catches first, then generic fallback
	processAudio.AddCatch(markFailed, &awsstepfunctions.CatchProps{
		Errors:     &[]*string{jsii.String("Lambda.ServiceException"), jsii.String("Lambda.AWSLambdaException"), jsii.String("Lambda.SdkClientException"), jsii.String("States.TaskFailed")},
		ResultPath: jsii.String("$.error"),
	})
	processAudio.AddCatch(markFailed, &awsstepfunctions.CatchProps{
		Errors:     &[]*string{jsii.String("States.ALL")},
		ResultPath: jsii.String("$.error"),
	})

	// Add retry for transient errors on ProcessAudio
	processAudio.AddRetry(&awsstepfunctions.RetryProps{
		Errors:      &[]*string{jsii.String("Lambda.ClientExecutionTimeoutException"), jsii.String("Lambda.ServiceException"), jsii.String("Lambda.AWSLambdaException"), jsii.String("Lambda.SdkClientException"), jsii.String("States.TaskFailed")},
		Interval:    awscdk.Duration_Seconds(jsii.Number(3)),
		MaxAttempts: jsii.Number(3),
		BackoffRate: jsii.Number(2.0),
	})

	// Choice state: ValidateInput - checks file extension before processing.
	// Design note: StringMatches is case-sensitive, so *.mp3 will not match .MP3.
	// This is acceptable because S3 object keys are typically lowercase and the
	// EventBridge event preserves the original key casing. The Lambda handler also
	// validates extensions (case-insensitive via strings.ToLower) as defense-in-depth,
	// but since the Choice state runs first, uppercase extensions are rejected early.
	validateInput := awsstepfunctions.NewChoice(stack, jsii.String("ValidateInput"), nil)

	// Define valid file extension conditions using StringMatches on $.detail.object.key
	validMp3 := awsstepfunctions.Condition_StringMatches(jsii.String("$.detail.object.key"), jsii.String("*.mp3"))
	validWav := awsstepfunctions.Condition_StringMatches(jsii.String("$.detail.object.key"), jsii.String("*.wav"))
	validM4a := awsstepfunctions.Condition_StringMatches(jsii.String("$.detail.object.key"), jsii.String("*.m4a"))
	validOgg := awsstepfunctions.Condition_StringMatches(jsii.String("$.detail.object.key"), jsii.String("*.ogg"))
	validFlac := awsstepfunctions.Condition_StringMatches(jsii.String("$.detail.object.key"), jsii.String("*.flac"))

	// Combine valid conditions with OR
	validExtension := awsstepfunctions.Condition_Or(validMp3, validWav, validM4a, validOgg, validFlac)

	// Build the success chain after validation: ProcessAudio -> Polly -> MarkCompleted -> NotifyCompleted
	processAudio.Next(pollyTask).Next(markCompleted).Next(notifyCompleted)

	// Wire the Choice: valid extensions proceed to ProcessAudio, invalid go to MarkFailed
	validateInput.When(validExtension, processAudio, nil).Otherwise(markFailed)

	// Chain: WriteInitialRecord -> ValidateInput (Choice)
	chain := awsstepfunctions.Chain_Start(writeInitialRecord).Next(validateInput)

	// Step Functions State Machine
	stateMachine := awsstepfunctions.NewStateMachine(stack, jsii.String("SleepAudioPipelineStateMachine"), &awsstepfunctions.StateMachineProps{
		DefinitionBody: awsstepfunctions.DefinitionBody_FromChainable(chain),
		Logs: &awsstepfunctions.LogOptions{
			Destination:          logGroup,
			Level:                awsstepfunctions.LogLevel_ALL,
			IncludeExecutionData: jsii.Bool(true),
		},
		TracingEnabled: jsii.Bool(true),
	})

	// Grant SNS publish permissions to the state machine
	completedTopic.GrantPublish(stateMachine)
	failedTopic.GrantPublish(stateMachine)

	// CloudWatch Alarm - StateMachine ExecutionsFailed
	stateMachineAlarm := awscloudwatch.NewAlarm(stack, jsii.String("StateMachineExecutionsFailedAlarm"), &awscloudwatch.AlarmProps{
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/States"),
			MetricName: jsii.String("ExecutionsFailed"),
			Statistic:  awscloudwatch.Stats_SUM(),
			DimensionsMap: &map[string]*string{
				"StateMachineArn": stateMachine.StateMachineArn(),
			},
			Period: awscdk.Duration_Minutes(jsii.Number(1)),
		}),
		Threshold:          jsii.Number(1),
		EvaluationPeriods:  jsii.Number(1),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		AlarmDescription:   jsii.String("Alarm when state machine executions fail"),
	})

	// CloudWatch Alarm - Lambda Errors
	lambdaAlarm := awscloudwatch.NewAlarm(stack, jsii.String("LambdaErrorsAlarm"), &awscloudwatch.AlarmProps{
		Metric: awscloudwatch.NewMetric(&awscloudwatch.MetricProps{
			Namespace:  jsii.String("AWS/Lambda"),
			MetricName: jsii.String("Errors"),
			Statistic:  awscloudwatch.Stats_SUM(),
			DimensionsMap: &map[string]*string{
				"FunctionName": processorLambda.FunctionName(),
			},
			Period: awscdk.Duration_Minutes(jsii.Number(5)),
		}),
		Threshold:          jsii.Number(1),
		EvaluationPeriods:  jsii.Number(1),
		ComparisonOperator: awscloudwatch.ComparisonOperator_GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
		AlarmDescription:   jsii.String("Alarm when Lambda function errors occur"),
	})

	// Wire both alarms to the failed SNS topic for notifications
	snsAction := awscloudwatchactions.NewSnsAction(failedTopic)
	stateMachineAlarm.AddAlarmAction(snsAction)
	lambdaAlarm.AddAlarmAction(snsAction)

	// EventBridge Rule - matches Object Created events from the input bucket
	rule := awsevents.NewRule(stack, jsii.String("InputBucketObjectCreatedRule"), &awsevents.RuleProps{
		EventPattern: &awsevents.EventPattern{
			Source:     &[]*string{jsii.String("aws.s3")},
			DetailType: &[]*string{jsii.String("Object Created")},
			Detail: &map[string]interface{}{
				"bucket": map[string]interface{}{
					"name": []interface{}{inputBucket.BucketName()},
				},
			},
		},
	})

	// Add the state machine as the rule target
	rule.AddTarget(awseventstargets.NewSfnStateMachine(stateMachine, nil))

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	envName := getEnvContext(app)
	stackID := "SleepAudioPipeline-" + envName

	NewCdkBaseStack(app, stackID, &CdkBaseStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	// Conditionally instantiate the pipeline stack when context 'pipeline' is 'true'
	pipelineCtx := app.Node().TryGetContext(jsii.String("pipeline"))
	if pipelineCtx != nil {
		if pipelineStr, ok := pipelineCtx.(string); ok && pipelineStr == "true" {
			NewPipelineStack(app, "SleepAudioPipelineCI", &PipelineStackProps{
				StackProps: awscdk.StackProps{
					Env: env(),
				},
				EnvName: envName,
			})
		}
	}

	app.Synth(nil)
}

// allowedEnvs defines the valid environment context values.
var allowedEnvs = map[string]bool{
	"dev":     true,
	"staging": true,
	"prod":    true,
}

// getEnvContext reads the 'env' context value from the CDK app and defaults to 'dev'.
// If the value is not in the allowed list (dev, staging, prod), it defaults to 'dev'
// and adds a CDK annotation warning.
func getEnvContext(app awscdk.App) string {
	envCtx := app.Node().TryGetContext(jsii.String("env"))
	if envCtx == nil {
		return "dev"
	}
	if envStr, ok := envCtx.(string); ok && envStr != "" {
		if allowedEnvs[envStr] {
			return envStr
		}
		awscdk.Annotations_Of(app).AddWarningV2(jsii.String("InvalidEnvContext"), jsii.String("invalid env context value '"+envStr+"'; defaulting to 'dev'. Allowed values: dev, staging, prod"))
		return "dev"
	}
	return "dev"
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	return nil
}
