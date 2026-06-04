package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
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

	// Step Functions DynamoDB PutItem task - write initial PROCESSING record
	writeInitialRecord := awsstepfunctionstasks.NewDynamoPutItem(stack, jsii.String("WriteInitialRecord"), &awsstepfunctionstasks.DynamoPutItemProps{
		Table: metadataTable,
		Item: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			"audioId":   awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key"))),
			"status":    awsstepfunctionstasks.DynamoAttributeValue_FromString(jsii.String("PROCESSING")),
			"bucket":    awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.bucket.name"))),
			"objectKey": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key"))),
			"createdAt": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.time"))),
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

	// Step Functions DynamoDB UpdateItem task - mark as COMPLETED
	markCompleted := awsstepfunctionstasks.NewDynamoUpdateItem(stack, jsii.String("MarkCompleted"), &awsstepfunctionstasks.DynamoUpdateItemProps{
		Table: metadataTable,
		Key: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			"audioId": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key"))),
		},
		ExpressionAttributeValues: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			":status":    awsstepfunctionstasks.DynamoAttributeValue_FromString(jsii.String("COMPLETED")),
			":updatedAt": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.time"))),
		},
		UpdateExpression: jsii.String("SET #s = :status, updatedAt = :updatedAt"),
		ExpressionAttributeNames: &map[string]*string{
			"#s": jsii.String("status"),
		},
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// Step Functions DynamoDB UpdateItem task - mark as FAILED (error handler)
	markFailed := awsstepfunctionstasks.NewDynamoUpdateItem(stack, jsii.String("MarkFailed"), &awsstepfunctionstasks.DynamoUpdateItemProps{
		Table: metadataTable,
		Key: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			"audioId": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.detail.object.key"))),
		},
		ExpressionAttributeValues: &map[string]awsstepfunctionstasks.DynamoAttributeValue{
			":status":    awsstepfunctionstasks.DynamoAttributeValue_FromString(jsii.String("FAILED")),
			":updatedAt": awsstepfunctionstasks.DynamoAttributeValue_FromString(awsstepfunctions.JsonPath_StringAt(jsii.String("$.time"))),
		},
		UpdateExpression: jsii.String("SET #s = :status, updatedAt = :updatedAt"),
		ExpressionAttributeNames: &map[string]*string{
			"#s": jsii.String("status"),
		},
		ResultPath: awsstepfunctions.JsonPath_DISCARD(),
	})

	// Add error handling: Polly task catches all errors and transitions to MarkFailed
	pollyTask.AddCatch(markFailed, &awsstepfunctions.CatchProps{
		ResultPath: jsii.String("$.error"),
	})

	// Chain: WriteInitialRecord -> PollyTask -> MarkCompleted
	chain := awsstepfunctions.Chain_Start(writeInitialRecord).Next(pollyTask).Next(markCompleted)

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
