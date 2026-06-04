package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
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
	})

	// Step Functions State Machine
	// Using Standard (default) type rather than Express. While Express is the eventual
	// target for high-throughput short-duration audio processing, Standard is appropriate
	// for the current skeleton because:
	// - Express has a 5-minute max execution duration
	// - Express provides at-least-once (not exactly-once) semantics
	// - Express has no execution history API (relies solely on CloudWatch Logs)
	// Switch to Express once the pipeline is proven and idempotency is handled.
	stateMachine := awsstepfunctions.NewStateMachine(stack, jsii.String("SleepAudioPipelineStateMachine"), &awsstepfunctions.StateMachineProps{
		DefinitionBody: awsstepfunctions.DefinitionBody_FromChainable(pollyTask),
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
