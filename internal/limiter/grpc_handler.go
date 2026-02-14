package limiter

// this will be used by other microservices to ping the target API
// will rate limi the microservices

import (
	"context"

	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
	"github.com/JesterSe7en/Sentinel-Go/internal/pb"
)

// GRPCHandler "implements" the interface defined in your proto file
type GRPCHandler struct {
	pb.UnimplementedRateLimiterServer
	engine *SentinelEngine
}

func NewGRPCHandler(e *SentinelEngine) *GRPCHandler {
	return &GRPCHandler{engine: e}
}

// Allow is the function that actually answers the gRPC request
func (h *GRPCHandler) Allow(ctx context.Context, req *pb.AllowRequest) (*pb.AllowResponse, error) {
	// allowed, err := h.engine.Allow(ctx, req.Key, []any{})
	// if err != nil {
	// 	return nil, err
	// }
	//
	return &pb.AllowResponse{
		Allowed:         false,
		RemainingTokens: 0, // You can expand your engine to return the actual count
	}, nil
}

func (h *GRPCHandler) ListAlgorithms(req *pb.ListAlgorithmsRequest) (*pb.ListAlgorithmsResponse, error) {
	return &pb.ListAlgorithmsResponse{
		Algorithms: h.engine.ListAlgorithm(),
	}, nil
}

func (h *GRPCHandler) UpdateAlgorithm(ctx context.Context, req *pb.UpdateAlgorithmRequest) (*pb.UpdateAlgorithmResponse, error) {
	algo, err := algorithm.ParseAlgorithm(req.Algo)
	if err != nil {
		return &pb.UpdateAlgorithmResponse{
			Success: false,
		}, err
	}

	err = h.engine.UpdateAlgorithm(ctx, algo)
	return &pb.UpdateAlgorithmResponse{
		Success: err != nil,
	}, err
}
