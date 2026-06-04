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

	// All S3 buckets must have public access blocked
	template.AllResourcesProperties(jsii.String("AWS::S3::Bucket"), map[string]interface{}{
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
	// and filter on the input bucket name (token value verified via AnyValue matcher)
	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"EventPattern": map[string]interface{}{
			"source":      []interface{}{"aws.s3"},
			"detail-type": []interface{}{"Object Created"},
			"detail": map[string]interface{}{
				"bucket": map[string]interface{}{
					"name": assertions.Match_AnyValue(),
				},
			},
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

func TestStateMachineExists(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// A Step Functions state machine must exist in the stack
	template.ResourceCountIs(jsii.String("AWS::StepFunctions::StateMachine"), jsii.Number(1))
}

func TestStateMachineDefinitionContainsPollyTask(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// The state machine DefinitionString is rendered as Fn::Join containing the Polly
	// task resource ARN. Verify the definition includes "polly:synthesizeSpeech" to
	// confirm the Polly integration is wired in.
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String("polly:synthesizeSpeech")),
				}),
			}),
		},
	})
}

func TestStateMachineHasLogging(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// The state machine must have LoggingConfiguration with level ALL
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"LoggingConfiguration": assertions.Match_ObjectLike(&map[string]interface{}{
			"Level":                "ALL",
			"IncludeExecutionData": true,
			"Destinations":         assertions.Match_AnyValue(),
		}),
	})
}

func TestEventBridgeRuleTargetsStateMachine(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// The EventBridge rule target Arn should reference the state machine
	template.HasResourceProperties(jsii.String("AWS::Events::Rule"), map[string]interface{}{
		"Targets": assertions.Match_ArrayWith(&[]interface{}{
			assertions.Match_ObjectLike(&map[string]interface{}{
				"Arn": map[string]interface{}{
					"Ref": assertions.Match_StringLikeRegexp(jsii.String("SleepAudioPipelineStateMachine")),
				},
			}),
		}),
	})
}

func TestStateMachineIAMRole(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// An IAM policy with polly:synthesizeSpeech permission must exist for the state machine role.
	// CDK may render individual statements per action or merge them; match the Polly action
	// as a string within the Statement array to cover both cases.
	template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]interface{}{
		"PolicyDocument": map[string]interface{}{
			"Statement": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ObjectLike(&map[string]interface{}{
					"Action": "polly:synthesizeSpeech",
					"Effect": "Allow",
				}),
			}),
		},
		"Roles": assertions.Match_ArrayWith(&[]interface{}{
			map[string]interface{}{
				"Ref": assertions.Match_StringLikeRegexp(jsii.String("SleepAudioPipelineStateMachine")),
			},
		}),
	})
}

func TestPlaceholderLambdaRemoved(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// The only Lambda function should be the CDK-internal BucketNotificationsHandler.
	// No user-defined placeholder Lambda should exist.
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(1))
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Description": assertions.Match_StringLikeRegexp(jsii.String("S3BucketNotifications")),
	})
}
