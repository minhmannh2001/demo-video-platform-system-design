#!/usr/bin/env bash
set -euo pipefail

ENDPOINT="${AWS_ENDPOINT_URL:-http://localhost:4566}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="${AWS_REGION:-us-east-1}"

echo "Waiting for LocalStack S3 at ${ENDPOINT}..."
for _ in $(seq 1 90); do
  if aws --endpoint-url="${ENDPOINT}" s3 ls >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

aws --endpoint-url="${ENDPOINT}" s3 mb "s3://video-raw" 2>/dev/null || true
aws --endpoint-url="${ENDPOINT}" s3 mb "s3://video-encoded" 2>/dev/null || true

if ! QUEUE_URL=$(aws --endpoint-url="${ENDPOINT}" sqs get-queue-url \
  --queue-name video-encode-jobs \
  --output text \
  --query 'QueueUrl' 2>/dev/null); then
  QUEUE_URL=$(aws --endpoint-url="${ENDPOINT}" sqs create-queue \
    --queue-name video-encode-jobs \
    --output text \
    --query 'QueueUrl')
fi

if ! META_QUEUE_URL=$(aws --endpoint-url="${ENDPOINT}" sqs get-queue-url \
  --queue-name video-metadata-index \
  --output text \
  --query 'QueueUrl' 2>/dev/null); then
  META_QUEUE_URL=$(aws --endpoint-url="${ENDPOINT}" sqs create-queue \
    --queue-name video-metadata-index \
    --output text \
    --query 'QueueUrl')
fi

echo "S3 buckets: video-raw, video-encoded"
echo "SQS encode queue URL: ${QUEUE_URL}"
echo "SQS metadata index queue URL: ${META_QUEUE_URL}"

# DLQ for metadata queue: after maxReceiveCount receives without DeleteMessage, message moves here.
if ! META_DLQ_URL=$(aws --endpoint-url="${ENDPOINT}" sqs get-queue-url \
  --queue-name video-metadata-index-dlq \
  --output text \
  --query QueueUrl 2>/dev/null); then
  META_DLQ_URL=$(aws --endpoint-url="${ENDPOINT}" sqs create-queue \
    --queue-name video-metadata-index-dlq \
    --output text \
    --query QueueUrl)
fi
META_DLQ_ARN=$(aws --endpoint-url="${ENDPOINT}" sqs get-queue-attributes \
  --queue-url "${META_DLQ_URL}" \
  --attribute-names QueueArn \
  --output text \
  --query Attributes.QueueArn)
aws --endpoint-url="${ENDPOINT}" sqs set-queue-attributes \
  --queue-url "${META_QUEUE_URL}" \
  --attributes "{\"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"${META_DLQ_ARN}\\\",\\\"maxReceiveCount\\\":\\\"5\\\"}\"}" \
  2>/dev/null || echo "warning: could not set redrive on video-metadata-index (check LocalStack SQS support)"
echo "SQS metadata DLQ URL: ${META_DLQ_URL}"

aws --endpoint-url="${ENDPOINT}" s3api put-bucket-cors --bucket video-raw --cors-configuration '{
  "CORSRules":[{"AllowedOrigins":["*"],"AllowedMethods":["GET","PUT","POST","HEAD"],"AllowedHeaders":["*"]}]
}' 2>/dev/null || true
aws --endpoint-url="${ENDPOINT}" s3api put-bucket-cors --bucket video-encoded --cors-configuration '{
  "CORSRules":[{"AllowedOrigins":["*"],"AllowedMethods":["GET","HEAD"],"AllowedHeaders":["*"]}]
}' 2>/dev/null || true
