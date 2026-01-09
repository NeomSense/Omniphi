package types

import (
	"github.com/cosmos/cosmos-sdk/types/query"
)

// Query request/response types

// QueryParamsRequest is the request for Query/Params
type QueryParamsRequest struct{}

// QueryParamsResponse is the response for Query/Params
type QueryParamsResponse struct {
	Params Params `json:"params"`
}

// QueryOperationRequest is the request for Query/Operation
type QueryOperationRequest struct {
	OperationId uint64 `json:"operation_id"`
}

// QueryOperationResponse is the response for Query/Operation
type QueryOperationResponse struct {
	Operation *QueuedOperation `json:"operation"`
}

// QueryOperationsRequest is the request for Query/Operations
type QueryOperationsRequest struct {
	Status     OperationStatus      `json:"status"`
	Pagination *query.PageRequest   `json:"pagination"`
}

// QueryOperationsResponse is the response for Query/Operations
type QueryOperationsResponse struct {
	Operations []QueuedOperation    `json:"operations"`
	Pagination *query.PageResponse  `json:"pagination"`
}

// QueryQueuedOperationsRequest is the request for Query/QueuedOperations
type QueryQueuedOperationsRequest struct {
	Pagination *query.PageRequest `json:"pagination"`
}

// QueryQueuedOperationsResponse is the response for Query/QueuedOperations
type QueryQueuedOperationsResponse struct {
	Operations []QueuedOperation   `json:"operations"`
	Pagination *query.PageResponse `json:"pagination"`
}

// QueryExecutableOperationsRequest is the request for Query/ExecutableOperations
type QueryExecutableOperationsRequest struct {
	Pagination *query.PageRequest `json:"pagination"`
}

// QueryExecutableOperationsResponse is the response for Query/ExecutableOperations
type QueryExecutableOperationsResponse struct {
	Operations []QueuedOperation   `json:"operations"`
	Pagination *query.PageResponse `json:"pagination"`
}

// QueryOperationByHashRequest is the request for Query/OperationByHash
type QueryOperationByHashRequest struct {
	Hash string `json:"hash"`
}

// QueryOperationByHashResponse is the response for Query/OperationByHash
type QueryOperationByHashResponse struct {
	Operation *QueuedOperation `json:"operation"`
}

// QueryOperationsByProposalRequest is the request for Query/OperationsByProposal
type QueryOperationsByProposalRequest struct {
	ProposalId uint64 `json:"proposal_id"`
}

// QueryOperationsByProposalResponse is the response for Query/OperationsByProposal
type QueryOperationsByProposalResponse struct {
	Operations []QueuedOperation `json:"operations"`
}
