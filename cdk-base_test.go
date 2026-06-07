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

func TestLambdaFunctionExists(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// Two Lambda functions: CDK BucketNotificationsHandler + SleepAudioProcessor
	template.ResourceCountIs(jsii.String("AWS::Lambda::Function"), jsii.Number(2))
}

func TestLambdaFunctionHasCorrectRuntime(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// SleepAudioProcessor Lambda must use provided.al2023 runtime (Go compiled binary)
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Runtime": "provided.al2023",
		"Handler": "bootstrap",
	})
}

func TestLambdaFunctionHasEnvironmentVariables(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// Lambda must have TABLE_NAME environment variable referencing the DynamoDB table
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"TABLE_NAME": assertions.Match_AnyValue(),
			},
		},
	})
}

func TestStateMachineDefinitionContainsLambdaInvoke(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must include a Lambda invoke task (arn:aws:states:::lambda:invoke)
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String("lambda:invoke")),
				}),
			}),
		},
	})
}

func TestStateMachineHasLambdaInvokePermission(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// IAM policy must grant lambda:InvokeFunction to the state machine role
	template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]interface{}{
		"PolicyDocument": map[string]interface{}{
			"Statement": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ObjectLike(&map[string]interface{}{
					"Action": "lambda:InvokeFunction",
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

func TestLambdaHasDynamoDBPermissions(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// Lambda execution role must have DynamoDB read/write permissions
	template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]interface{}{
		"PolicyDocument": map[string]interface{}{
			"Statement": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ObjectLike(&map[string]interface{}{
					"Action": assertions.Match_ArrayWith(&[]interface{}{
						"dynamodb:BatchGetItem",
						"dynamodb:BatchWriteItem",
						"dynamodb:PutItem",
						"dynamodb:UpdateItem",
					}),
					"Effect": "Allow",
				}),
			}),
		},
		"Roles": assertions.Match_ArrayWith(&[]interface{}{
			map[string]interface{}{
				"Ref": assertions.Match_StringLikeRegexp(jsii.String("SleepAudioProcessor")),
			},
		}),
	})
}

func TestDynamoDBTableExists(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// DynamoDB table must exist with audioId partition key (S), PAY_PER_REQUEST billing,
	// server-side encryption enabled, and point-in-time recovery enabled.
	template.HasResourceProperties(jsii.String("AWS::DynamoDB::Table"), map[string]interface{}{
		"KeySchema": []interface{}{
			map[string]interface{}{
				"AttributeName": "audioId",
				"KeyType":       "HASH",
			},
		},
		"AttributeDefinitions": []interface{}{
			map[string]interface{}{
				"AttributeName": "audioId",
				"AttributeType": "S",
			},
		},
		"BillingMode": "PAY_PER_REQUEST",
		"SSESpecification": map[string]interface{}{
			"SSEEnabled": true,
		},
		"PointInTimeRecoverySpecification": map[string]interface{}{
			"PointInTimeRecoveryEnabled": true,
		},
	})
}

func TestStateMachineDefinitionContainsDynamoDBPutItem(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must include a DynamoDB PutItem task
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String("dynamodb:putItem")),
				}),
			}),
		},
	})
}

func TestStateMachineDefinitionContainsDynamoDBUpdateItem(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must include a DynamoDB UpdateItem task
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String("dynamodb:updateItem")),
				}),
			}),
		},
	})
}

func TestStateMachineHasDynamoDBPermissions(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// IAM policy must grant dynamodb:PutItem and dynamodb:UpdateItem to the state machine role
	template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]interface{}{
		"PolicyDocument": map[string]interface{}{
			"Statement": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ObjectLike(&map[string]interface{}{
					"Action": "dynamodb:PutItem",
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

	template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]interface{}{
		"PolicyDocument": map[string]interface{}{
			"Statement": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ObjectLike(&map[string]interface{}{
					"Action": "dynamodb:UpdateItem",
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

func TestStateMachineDefinitionHasErrorHandling(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must include error handling (Catch keyword)
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String("Catch")),
				}),
			}),
		},
	})
}

func TestSNSTopicsExist(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// Exactly 2 SNS topics must exist (Completed and Failed)
	template.ResourceCountIs(jsii.String("AWS::SNS::Topic"), jsii.Number(2))
}

func TestSNSTopicsHaveEncryption(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// All SNS topics must have KMS encryption configured
	template.AllResourcesProperties(jsii.String("AWS::SNS::Topic"), map[string]interface{}{
		"KmsMasterKeyId": assertions.Match_AnyValue(),
	})
}

func TestStateMachineDefinitionContainsSNSPublish(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must contain sns:publish indicating SNS publish tasks
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String("sns:publish")),
				}),
			}),
		},
	})
}

func TestStateMachineHasSNSPublishPermissions(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// IAM policy must grant sns:Publish to the state machine role
	template.HasResourceProperties(jsii.String("AWS::IAM::Policy"), map[string]interface{}{
		"PolicyDocument": map[string]interface{}{
			"Statement": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ObjectLike(&map[string]interface{}{
					"Action": "sns:Publish",
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

func TestStateMachineDefinitionHasCompletedNotification(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must contain NotifyCompleted state name
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String("NotifyCompleted")),
				}),
			}),
		},
	})
}

func TestStateMachineDefinitionHasFailedNotification(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must contain NotifyFailed state name
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String("NotifyFailed")),
				}),
			}),
		},
	})
}

func TestMarkCompletedTransitionsToNotifyCompleted(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// MarkCompleted state must transition to NotifyCompleted via Next field
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String(`MarkCompleted.*"Next".*NotifyCompleted`)),
				}),
			}),
		},
	})
}

func TestMarkFailedTransitionsToNotifyFailed(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// MarkFailed state must transition to NotifyFailed via Next field
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String(`MarkFailed.*"Next".*NotifyFailed`)),
				}),
			}),
		},
	})
}

func TestSNSTopicsHaveNoBoardcodedNames(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// SNS topics must NOT have hardcoded TopicName - let CDK generate unique names
	template.AllResourcesProperties(jsii.String("AWS::SNS::Topic"), map[string]interface{}{
		"TopicName": assertions.Match_Absent(),
	})
}

func TestStateMachineDefinitionContainsValidateInput(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must include a ValidateInput state
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String("ValidateInput")),
				}),
			}),
		},
	})
}

func TestStateMachineDefinitionContainsChoiceState(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must include a Choice state type for input validation
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String(`"Type":"Choice"`)),
				}),
			}),
		},
	})
}

func TestStateMachineValidationChecksFileExtensions(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// State machine definition must include StringMatches patterns for valid audio extensions
	extensions := []string{".mp3", ".wav", ".m4a", ".ogg", ".flac"}
	for _, ext := range extensions {
		template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
			"DefinitionString": map[string]interface{}{
				"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_ArrayWith(&[]interface{}{
						assertions.Match_StringLikeRegexp(jsii.String("\\*\\" + ext)),
					}),
				}),
			},
		})
	}
}

func TestStateMachineDefinitionContainsValidationErrorPath(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// The Choice state Default (otherwise) must route to MarkFailed,
	// meaning the definition contains a Default pointing to MarkFailed
	template.HasResourceProperties(jsii.String("AWS::StepFunctions::StateMachine"), map[string]interface{}{
		"DefinitionString": map[string]interface{}{
			"Fn::Join": assertions.Match_ArrayWith(&[]interface{}{
				assertions.Match_ArrayWith(&[]interface{}{
					assertions.Match_StringLikeRegexp(jsii.String(`"Default":"MarkFailed"`)),
				}),
			}),
		},
	})
}

func TestLambdaFunctionHasOutputBucketEnv(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewCdkBaseStack(app, "TestStack", nil)
	template := assertions.Template_FromStack(stack, nil)

	// Lambda must have OUTPUT_BUCKET_NAME environment variable
	template.HasResourceProperties(jsii.String("AWS::Lambda::Function"), map[string]interface{}{
		"Environment": map[string]interface{}{
			"Variables": map[string]interface{}{
				"OUTPUT_BUCKET_NAME": assertions.Match_AnyValue(),
			},
		},
	})
}
