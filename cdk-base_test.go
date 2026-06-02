package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

func TestCdkBaseStack(t *testing.T) {
	defer jsii.Close()

	// GIVEN
	app := awscdk.NewApp(nil)

	// WHEN
	stack := NewCdkBaseStack(app, "MyStack", nil)

	// THEN - stack synthesizes without error
	template := assertions.Template_FromStack(stack, nil)

	// Verify no SQS queues are created (sanity check)
	template.ResourceCountIs(jsii.String("AWS::SQS::Queue"), jsii.Number(0))
}

func TestInputBucketExists(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// Input bucket must have S3-managed encryption and versioning enabled
	template.HasResourceProperties(jsii.String("AWS::S3::Bucket"), map[string]interface{}{
		"BucketEncryption": map[string]interface{}{
			"ServerSideEncryptionConfiguration": []interface{}{
				map[string]interface{}{
					"ServerSideEncryptionByDefault": map[string]interface{}{
						"SSEAlgorithm": "AES256",
					},
				},
			},
		},
		"VersioningConfiguration": map[string]interface{}{
			"Status": "Enabled",
		},
	})

	// EventBridge notifications are enabled via a Custom::S3BucketNotifications resource
	template.HasResourceProperties(jsii.String("Custom::S3BucketNotifications"), map[string]interface{}{
		"NotificationConfiguration": map[string]interface{}{
			"EventBridgeConfiguration": map[string]interface{}{},
		},
	})
}

func TestOutputBucketExists(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// There should be at least 2 S3 buckets (input and output)
	template.ResourceCountIs(jsii.String("AWS::S3::Bucket"), jsii.Number(2))
}

func TestBucketsHaveBlockPublicAccess(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// Both buckets should have public access blocked
	template.HasResourceProperties(jsii.String("AWS::S3::Bucket"), map[string]interface{}{
		"PublicAccessBlockConfiguration": map[string]interface{}{
			"BlockPublicAcls":       true,
			"BlockPublicPolicy":     true,
			"IgnorePublicAcls":      true,
			"RestrictPublicBuckets": true,
		},
	})
}

func TestEventBridgeRuleExists(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// EventBridge rule must match Object Created events from aws.s3 source
	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"EventPattern": map[string]interface{}{
			"source":      []interface{}{"aws.s3"},
			"detail-type": []interface{}{"Object Created"},
		},
	})
}

func TestEventBridgeRuleHasTarget(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// Rule must have at least one target
	template.ResourceCountIs(jsii.String("AWS::Events::Rule"), jsii.Number(1))

	// The rule should have Targets defined
	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"Targets": assertions.Match_AnyValue(),
	})
}

func TestPlaceholderLambdaExists(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// A placeholder Lambda function should exist as EventBridge rule target
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Runtime": "nodejs18.x",
		"Handler": "index.handler",
	})
}
