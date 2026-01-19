// Package server - Node service stub
package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/xs-wallet/xscore/internal/config"
	pb "github.com/xs-wallet/xscore/proto"
)

// =============================================================================
// NODE SERVICE
// =============================================================================

// NodeService implementa pb.NodeServiceServer
type NodeService struct {
	pb.UnimplementedNodeServiceServer
	cfg *config.Config
}

// NewNodeService cria NodeService
func NewNodeService(cfg *config.Config) *NodeService {
	return &NodeService{cfg: cfg}
}

// StartNode inicia node
func (s *NodeService) StartNode(ctx context.Context, req *pb.StartNodeRequest) (*pb.NodeStatus, error) {
	// TODO: Spawn processo do node
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// StopNode para node
func (s *NodeService) StopNode(ctx context.Context, req *pb.StopNodeRequest) (*pb.NodeStatus, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// RestartNode reinicia node
func (s *NodeService) RestartNode(ctx context.Context, req *pb.RestartNodeRequest) (*pb.NodeStatus, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// GetNodeStatus retorna status do node
func (s *NodeService) GetNodeStatus(ctx context.Context, req *pb.GetNodeStatusRequest) (*pb.NodeStatus, error) {
	return &pb.NodeStatus{
		NodeType: req.NodeType,
		State:    pb.NodeState_NODE_STATE_STOPPED,
	}, nil
}

// GetAllNodeStatuses retorna status de todos os nodes
func (s *NodeService) GetAllNodeStatuses(ctx context.Context, req *pb.GetAllNodeStatusesRequest) (*pb.AllNodeStatusesResponse, error) {
	return &pb.AllNodeStatusesResponse{
		Bitcoind:  &pb.NodeStatus{NodeType: pb.NodeType_NODE_TYPE_BITCOIND, State: pb.NodeState_NODE_STATE_STOPPED},
		Elementsd: &pb.NodeStatus{NodeType: pb.NodeType_NODE_TYPE_ELEMENTSD, State: pb.NodeState_NODE_STATE_STOPPED},
		Lnd:       &pb.NodeStatus{NodeType: pb.NodeType_NODE_TYPE_LND, State: pb.NodeState_NODE_STATE_STOPPED},
	}, nil
}

// GetSyncProgress retorna progresso de sync
func (s *NodeService) GetSyncProgress(ctx context.Context, req *pb.GetSyncProgressRequest) (*pb.SyncProgress, error) {
	return &pb.SyncProgress{
		NodeType: req.NodeType,
		Synced:   false,
	}, nil
}

// WatchNodeStatus streaming de status updates
func (s *NodeService) WatchNodeStatus(req *pb.WatchNodeStatusRequest, stream pb.NodeService_WatchNodeStatusServer) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// WatchLogs streaming de logs
func (s *NodeService) WatchLogs(req *pb.WatchLogsRequest, stream pb.NodeService_WatchLogsServer) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// CheckBinaryStatus verifica status do binário
func (s *NodeService) CheckBinaryStatus(ctx context.Context, req *pb.CheckBinaryStatusRequest) (*pb.BinaryStatus, error) {
	return &pb.BinaryStatus{
		NodeType:  req.NodeType,
		Installed: false,
	}, nil
}

// DownloadBinary faz download do binário
func (s *NodeService) DownloadBinary(req *pb.DownloadBinaryRequest, stream pb.NodeService_DownloadBinaryServer) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// VerifyBinary verifica integridade do binário
func (s *NodeService) VerifyBinary(ctx context.Context, req *pb.VerifyBinaryRequest) (*pb.VerifyBinaryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}
