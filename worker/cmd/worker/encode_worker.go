package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"video-platform/internal/tracing"
	"video-platform/internal/worker"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel/attribute"
)

const (
	encodeWorkerCount = 3
	encodeJobQueueCap = 32
)

type encodeJob struct {
	body    string
	receipt string
	msgID   string
	attrs   map[string]types.MessageAttributeValue
}

type encodeMessageProcessor interface {
	HandleMessage(ctx context.Context, body string) error
}

type sqsDeleteClient interface {
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

type encodeJobPool struct {
	ctx             context.Context
	proc            encodeMessageProcessor
	sqsClient       sqsDeleteClient
	queueURL        string
	jobQueue        <-chan encodeJob
	idleWorkerQueue chan chan encodeJob // queue of available workers (their private inbox channel)
	workerWG        sync.WaitGroup
	dispatcherWG    sync.WaitGroup
}

func newEncodeJobPool(ctx context.Context, proc encodeMessageProcessor, sqsClient sqsDeleteClient, queueURL string, jobQueue <-chan encodeJob) *encodeJobPool {
	return &encodeJobPool{
		ctx:             ctx,
		proc:            proc,
		sqsClient:       sqsClient,
		queueURL:        queueURL,
		jobQueue:        jobQueue,
		idleWorkerQueue: make(chan chan encodeJob, encodeWorkerCount),
	}
}

func (p *encodeJobPool) Start() {
	for i := 0; i < encodeWorkerCount; i++ {
		p.workerWG.Add(1)
		go p.runWorker(i + 1)
	}
	p.dispatcherWG.Add(1)
	go p.runDispatcher()
}

func (p *encodeJobPool) Wait() {
	p.dispatcherWG.Wait()
	p.workerWG.Wait()
}

func (p *encodeJobPool) runDispatcher() {
	defer p.dispatcherWG.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobQueue:
			if !ok {
				return
			}
			select {
			case <-p.ctx.Done():
				return
			case workerCh := <-p.idleWorkerQueue:
				workerCh <- job
			}
		}
	}
}

func (p *encodeJobPool) runWorker(workerID int) {
	defer p.workerWG.Done()
	inbox := make(chan encodeJob)
	for {
		// register as idle; dispatcher can now assign a job.
		select {
		case <-p.ctx.Done():
			return
		case p.idleWorkerQueue <- inbox:
		}
		select {
		case <-p.ctx.Done():
			return
		case job := <-inbox:
			p.processJob(workerID, job)
		}
	}
}

func (p *encodeJobPool) processJob(workerID int, job encodeJob) {
	msgCtx := tracing.ExtractFromSQSAttributes(p.ctx, job.attrs)
	var parsed struct {
		VideoID string `json:"video_id"`
	}
	spanAttrs := []attribute.KeyValue{
		attribute.String("messaging.system", "aws_sqs"),
		attribute.Int("worker.id", workerID),
	}
	if err := json.Unmarshal([]byte(job.body), &parsed); err == nil && parsed.VideoID != "" {
		spanAttrs = append(spanAttrs, attribute.String("video.id", parsed.VideoID))
	}
	if parsed.VideoID != "" {
		slog.InfoContext(msgCtx, "sqs_job_received",
			"video_id", parsed.VideoID,
			"sqs_message_id", job.msgID,
			"worker_id", workerID,
		)
	}
	msgCtx, span := tracing.Start(msgCtx, "worker.encode_job", spanAttrs...)
	procErr := p.proc.HandleMessage(msgCtx, job.body)
	tracing.Finish(span, procErr)
	if procErr != nil {
		slog.ErrorContext(msgCtx, "encode_job_failed", "error", procErr, "video_id", parsed.VideoID, "worker_id", workerID)
	}
	_, delErr := p.sqsClient.DeleteMessage(p.ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.queueURL),
		ReceiptHandle: aws.String(job.receipt),
	})
	if delErr != nil {
		slog.WarnContext(msgCtx, "sqs_delete_message_failed", "error", delErr, "worker_id", workerID)
	}
}

func runEncodeWorker(ctx context.Context, sqsClient *sqs.Client, queueURL string, proc *worker.Processor) {
	slog.Info("encoder worker polling", "queue_url", queueURL)
	jobQueue := make(chan encodeJob, encodeJobQueueCap)
	pool := newEncodeJobPool(ctx, proc, sqsClient, queueURL, jobQueue)
	pool.Start()
	defer func() {
		close(jobQueue)
		pool.Wait()
	}()

	for {
		if ctx.Err() != nil {
			slog.Info("encoder worker shutdown", "reason", ctx.Err().Error())
			return
		}
		out, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(queueURL),
			MaxNumberOfMessages:   10,
			WaitTimeSeconds:       20,
			VisibilityTimeout:     300,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("sqs receive failed", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, msg := range out.Messages {
			j := encodeJob{
				body:    aws.ToString(msg.Body),
				receipt: aws.ToString(msg.ReceiptHandle),
				msgID:   aws.ToString(msg.MessageId),
				attrs:   msg.MessageAttributes,
			}
			select {
			case <-ctx.Done():
				return
			case jobQueue <- j:
			}
		}
	}
}
