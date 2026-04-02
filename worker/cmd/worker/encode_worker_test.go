package main

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type fakeProc struct {
	calls []string
	err   error
}

func (f *fakeProc) HandleMessage(ctx context.Context, body string) error {
	f.calls = append(f.calls, body)
	return f.err
}

type fakeSQSDelete struct {
	receipts []string
}

func (f *fakeSQSDelete) DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.receipts = append(f.receipts, aws.ToString(in.ReceiptHandle))
	return &sqs.DeleteMessageOutput{}, nil
}

func TestEncodeJobPool_DispatcherWaitsForIdleWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	jobQueue := make(chan encodeJob, 1)
	p := newEncodeJobPool(ctx, &fakeProc{}, &fakeSQSDelete{}, "q", jobQueue)
	p.dispatcherWG.Add(1)
	go p.runDispatcher()
	defer func() {
		cancel()
		p.dispatcherWG.Wait()
	}()

	job := encodeJob{body: `{"video_id":"v1"}`}
	jobQueue <- job

	workerCh := make(chan encodeJob, 1)
	select {
	case got := <-workerCh:
		t.Fatalf("job should not be delivered before worker is idle: %+v", got)
	case <-time.After(80 * time.Millisecond):
		// expected: dispatcher blocks waiting on idleWorkerQueue
	}

	p.idleWorkerQueue <- workerCh
	select {
	case got := <-workerCh:
		if got.body != job.body {
			t.Fatalf("job body = %q, want %q", got.body, job.body)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("timed out waiting for dispatched job")
	}
}

func TestEncodeJobPool_ProcessJobCallsProcessorAndDeletesMessage(t *testing.T) {
	ctx := context.Background()
	fp := &fakeProc{}
	fs := &fakeSQSDelete{}
	p := newEncodeJobPool(ctx, fp, fs, "http://queue", make(chan encodeJob))

	job := encodeJob{
		body:    `{"video_id":"abc"}`,
		receipt: "rh-1",
		msgID:   "m-1",
	}
	p.processJob(2, job)

	if len(fp.calls) != 1 || fp.calls[0] != job.body {
		t.Fatalf("processor calls = %#v", fp.calls)
	}
	if len(fs.receipts) != 1 || fs.receipts[0] != "rh-1" {
		t.Fatalf("delete receipts = %#v", fs.receipts)
	}
}

