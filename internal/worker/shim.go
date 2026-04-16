package worker

import (
	"context"

	"github.com/Keyhole-Koro/SynthifyShared/domain"
	"github.com/synthify/backend/internal/jobstatus"
	workerpkg "github.com/synthify/backend/worker/pkg/worker"
	"github.com/synthify/backend/worker/pkg/worker/pipeline"
)

type DispatchRequest = workerpkg.DispatchRequest
type Dispatcher = workerpkg.Dispatcher
type InlineDispatcher = workerpkg.InlineDispatcher
type HTTPDispatcher = workerpkg.HTTPDispatcher
type InternalHandler = workerpkg.InternalHandler
type Processor = workerpkg.Processor

func NewInlineDispatcher(processor interface {
	Process(ctx context.Context, pctx *pipeline.PipelineContext) error
}) *InlineDispatcher {
	return workerpkg.NewInlineDispatcher(processor)
}

func NewHTTPDispatcher(baseURL, token string) *HTTPDispatcher {
	return workerpkg.NewHTTPDispatcher(baseURL, token)
}

func NewInternalHandler(processor interface {
	Process(ctx context.Context, pctx *pipeline.PipelineContext) error
}, token string) *InternalHandler {
	return workerpkg.NewInternalHandler(processor, token)
}

func NewProcessor(jobRepo interface {
	MarkProcessingJobRunning(jobID string) bool
	UpdateProcessingJobStage(jobID, stage string) bool
	FailProcessingJob(jobID, errorMessage string) bool
	CompleteProcessingJob(jobID string) bool
	SaveDocumentChunks(documentID string, chunks []*domain.DocumentChunk) error
}, graphRepo interface {
	CreateStructuredNode(graphID, label, category string, level int, entityType, description, summaryHTML, createdBy string) *domain.Node
	CreateEdge(graphID, sourceNodeID, targetNodeID, edgeType, description string) *domain.Edge
	UpsertNodeSource(nodeID, documentID, chunkID, sourceText string, confidence float64) error
	UpsertEdgeSource(edgeID, documentID, chunkID, sourceText string, confidence float64) error
	UpdateNodeSummaryHTML(nodeID, summaryHTML string) bool
}) *Processor {
	return workerpkg.NewProcessor(jobRepo, graphRepo)
}

func NewProcessorWithNotifier(jobRepo interface {
	MarkProcessingJobRunning(jobID string) bool
	UpdateProcessingJobStage(jobID, stage string) bool
	FailProcessingJob(jobID, errorMessage string) bool
	CompleteProcessingJob(jobID string) bool
	SaveDocumentChunks(documentID string, chunks []*domain.DocumentChunk) error
}, graphRepo interface {
	CreateStructuredNode(graphID, label, category string, level int, entityType, description, summaryHTML, createdBy string) *domain.Node
	CreateEdge(graphID, sourceNodeID, targetNodeID, edgeType, description string) *domain.Edge
	UpsertNodeSource(nodeID, documentID, chunkID, sourceText string, confidence float64) error
	UpsertEdgeSource(edgeID, documentID, chunkID, sourceText string, confidence float64) error
	UpdateNodeSummaryHTML(nodeID, summaryHTML string) bool
}, notifier jobstatus.Notifier) *Processor {
	return workerpkg.NewProcessorWithNotifier(jobRepo, graphRepo, notifier)
}
