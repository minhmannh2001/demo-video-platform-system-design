#!/usr/bin/env bash
# Test cases: Infra B/C — S3 buckets and SQS queue exist after init-localstack.sh
set -euo pipefail
ENDPOINT="${AWS_ENDPOINT_URL:-http://localhost:4566}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="${AWS_REGION:-us-east-1}"

aws --endpoint-url="${ENDPOINT}" s3 ls "s3://video-raw" >/dev/null
aws --endpoint-url="${ENDPOINT}" s3 ls "s3://video-encoded" >/dev/null
aws --endpoint-url="${ENDPOINT}" sqs get-queue-url --queue-name video-encode-jobs --output text --query QueueUrl >/dev/null
echo "verify-localstack: OK"
