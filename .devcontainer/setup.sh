#!/usr/bin/env bash
set -euo pipefail

cd /workspaces/"$(basename "$PWD")" || exit 1

npm install -g aws-cdk

if [ ! -f "go.mod" ]; then
  go mod init example.com/cdk-go-experiment
fi

if [ ! -f "cdk.json" ]; then
  cdk init app --language go
fi

go mod tidy
