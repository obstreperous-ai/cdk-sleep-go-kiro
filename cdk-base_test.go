package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

// TestCdkBaseStack is a baseline synthesis validation proving the CDK jsii
// bridge and stack definition work end-to-end. It will be replaced with
// resource-specific assertions as infrastructure is added.
func TestCdkBaseStack(t *testing.T) {
	defer jsii.Close()

	// GIVEN
	app := awscdk.NewApp(nil)

	// WHEN
	stack := NewCdkBaseStack(app, "MyStack", nil)

	// THEN
	template := assertions.Template_FromStack(stack, nil)

	template.ResourceCountIs(jsii.String("AWS::SQS::Queue"), jsii.Number(0))
}
