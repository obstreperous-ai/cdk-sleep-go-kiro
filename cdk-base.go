package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
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
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
		Versioned:         jsii.Bool(true),
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		EventBridgeEnabled: jsii.Bool(true),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
	})

	// S3 Output Bucket - stores processed audio artifacts
	awss3.NewBucket(stack, jsii.String("SleepAudioOutputBucket"), &awss3.BucketProps{
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
		Versioned:         jsii.Bool(true),
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
	})

	// Placeholder Lambda - stub target for EventBridge rule (will be replaced by Step Functions)
	placeholderFn := awslambda.NewFunction(stack, jsii.String("PlaceholderProcessorFn"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_NODEJS_18_X(),
		Handler: jsii.String("index.handler"),
		Code:    awslambda.Code_FromInline(jsii.String("exports.handler = async () => { return { statusCode: 200 }; };")),
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

	// Add the placeholder Lambda as the rule target
	rule.AddTarget(awseventstargets.NewLambdaFunction(placeholderFn, nil))

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
