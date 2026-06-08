package main

import (
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/pipelines"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type PipelineStackProps struct {
	awscdk.StackProps
	EnvName string
}

// NewPipelineStack creates a CDK Pipelines CodePipeline stack for CI/CD.
func NewPipelineStack(scope constructs.Construct, id string, props *PipelineStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	envName := "dev"
	if props != nil && props.EnvName != "" {
		envName = props.EnvName
	}

	// GitHub source connection (placeholder ARN - replace with actual CodeStar connection)
	connectionArn := "arn:aws:codestar-connections:us-east-1:123456789012:connection/placeholder"
	source := pipelines.CodePipelineSource_Connection(
		jsii.String("owner/cdk-sleep-go-kiro"),
		jsii.String("main"),
		&pipelines.ConnectionSourceOptions{
			ConnectionArn: jsii.String(connectionArn),
		},
	)

	// Guard: fail synthesis if the connection ARN still contains "placeholder"
	if strings.Contains(connectionArn, "placeholder") {
		awscdk.Annotations_Of(stack).AddError(jsii.String("Pipeline connection ARN contains 'placeholder'. Replace with a real CodeStar connection ARN before deploying."))
	}

	// Synth step: download modules, run tests, and synthesize CDK
	synthStep := pipelines.NewShellStep(jsii.String("Synth"), &pipelines.ShellStepProps{
		Input: source,
		Commands: &[]*string{
			jsii.String("go mod download"),
			jsii.String("GOPROXY=direct go test ./..."),
			jsii.String("npx cdk synth"),
		},
	})

	// Create the CodePipeline
	pipeline := pipelines.NewCodePipeline(stack, jsii.String("SleepAudioPipeline"), &pipelines.CodePipelineProps{
		PipelineName: jsii.String("SleepAudioPipeline-" + envName),
		Synth:        synthStep,
	})

	// Add a deployment stage for the application stack
	deployStage := awscdk.NewStage(stack, jsii.String("Deploy-"+envName), &awscdk.StageProps{})
	NewCdkBaseStack(deployStage, "SleepAudioPipeline-"+envName, nil)

	pipeline.AddStage(deployStage, nil)

	return stack
}
