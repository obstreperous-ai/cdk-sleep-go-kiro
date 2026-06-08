package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

func TestPipelineStackExists(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewPipelineStack(app, "TestPipelineStack", &PipelineStackProps{
		EnvName: "dev",
	})
	template := assertions.Template_FromStack(stack, nil)

	// Pipeline stack must contain a CodePipeline resource
	template.ResourceCountIs(jsii.String("AWS::CodePipeline::Pipeline"), jsii.Number(1))
}

func TestPipelineHasSourceStage(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewPipelineStack(app, "TestPipelineStack", &PipelineStackProps{
		EnvName: "dev",
	})
	template := assertions.Template_FromStack(stack, nil)

	// Pipeline must have a Source stage with CodeStarSourceConnection action
	template.HasResourceProperties(jsii.String("AWS::CodePipeline::Pipeline"), map[string]interface{}{
		"Stages": assertions.Match_ArrayWith(&[]interface{}{
			assertions.Match_ObjectLike(&map[string]interface{}{
				"Name": "Source",
			}),
		}),
	})
}

func TestPipelineHasSynthStep(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	stack := NewPipelineStack(app, "TestPipelineStack", &PipelineStackProps{
		EnvName: "dev",
	})
	template := assertions.Template_FromStack(stack, nil)

	// Pipeline must have a Build stage (synth step)
	template.HasResourceProperties(jsii.String("AWS::CodePipeline::Pipeline"), map[string]interface{}{
		"Stages": assertions.Match_ArrayWith(&[]interface{}{
			assertions.Match_ObjectLike(&map[string]interface{}{
				"Name": "Build",
			}),
		}),
	})
}
