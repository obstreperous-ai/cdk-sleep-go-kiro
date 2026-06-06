package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
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
	awss3.NewBucket(stack, jsii.String("SleepAudioOutputBucket"), &awss3.BucketProps{
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
		PointInTimeRecovery: jsii.Bool(true),
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
		Environment: &map[string]*string{
			"TABLE_NAME": metadataTable.TableName(),
		},
	})

	// Grant Lambda read/write access to the DynamoDB metadata table
	metadataTable.GrantReadWriteData(processorLambda)

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
	processAudio := awsstepfunctionstasks.NewLambdaInvoke(stack, jsii.String("ProcessAudio"), &awsstepfunctionstasks.LambdaInvokeProps{
		LambdaFunction: processorLambda,
		Payload: awsstepfunctions.TaskInput_FromObject(&map[string]interface{}{
			"audioId":   awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key")),
			"bucket":    awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.bucket.name")),
			"objectKey": awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key")),
		}),
		ResultPath: jsii.String("$.processorResult"),
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

	// Add error handling: Polly task catches all errors and transitions to MarkFailed
	pollyTask.AddCatch(markFailed, &awsstepfunctions.CatchProps{
		ResultPath: jsii.String("$.error"),
	})

	// Add error handling: ProcessAudio Lambda catches all errors and transitions to MarkFailed
	processAudio.AddCatch(markFailed, &awsstepfunctions.CatchProps{
		ResultPath: jsii.String("$.error"),
	})

	// Chain: WriteInitialRecord -> ProcessAudio (Lambda) -> PollyTask -> MarkCompleted -> NotifyCompleted
	chain := awsstepfunctions.Chain_Start(writeInitialRecord).Next(processAudio).Next(pollyTask).Next(markCompleted).Next(notifyCompleted)

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

	NewCdkBaseStack(app, "CdkBaseStack", &CdkBaseStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	return nil
}
