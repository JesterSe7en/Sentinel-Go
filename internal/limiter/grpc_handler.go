package limiter

// this will be used by other microservices to ping the target API
// will rate limi the microservices

import (
	"context"

	"github.com/JesterSe7en/Sentinel-Go/api/v1/pb"
	"github.com/JesterSe7en/Sentinel-Go/internal/algorithm"
	"github.com/prometheus/client_golang/prometheus"
)

type GRPCMetrics struct {
	grpcAllowRequestTotal *prometheus.CounterVec
}

func registerGRPCMetrics(reg prometheus.Registerer) *GRPCMetrics {
	return &GRPCMetrics{
		grpcAllowRequestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "grpc_allow_request_total",
				Help: "Total number of requests allowed via gRPC.",
			},
			[]string{"decision", "algorithm"},
		),
	}
}

// GRPCHandler "implements" the interface dfined in your proto file
type GRPCHandler struct {
	pb.UnimplementedRateLimiterServiceServer
	engine *SentinelEngine
}

func NewGRPCHandler(e *SentinelEngine) *GRPCHandler {
	return &GRPCHandler{engine: e}
}

// Allow is the function that actually answers the gRPC request
func (h *GRPCHandler) Allow(ctx context.Context, req *pb.AllowRequest) (*pb.AllowResponse, error) {
	results, err := h.engine.Allow(ctx, req.Key)
	return &pb.AllowResponse{
		Allowed:   results.Allowed,
		Limit:     int32(results.Limit),
		Remaining: int32(results.Remaining),
		Reset_:    int32(results.Reset),
	}, err
}

func (h *GRPCHandler) ListAlgorithms(ctx context.Context, req *pb.ListAlgorithmsRequest) (*pb.ListAlgorithmsResponse, error) {
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

func (h *GRPCHandler) GetCurrentAlgorithm(ctx context.Context, req *pb.GetCurrentAlgorithmRequest) (*pb.GetCurrentAlgorithmResponse, error) {
	algo, err := h.engine.GetCurrentAlgorithm(ctx)
	if err != nil {
		return &pb.GetCurrentAlgorithmResponse{
			Algorithm: "",
		}, err
	}
	return &pb.GetCurrentAlgorithmResponse{
		Algorithm: algo,
	}, nil
}
