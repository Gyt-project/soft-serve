package grpc

import (
	"context"
	"net"

	"charm.land/log/v2"
	"github.com/Gyt-project/soft-serve/pkg/backend"
	"github.com/Gyt-project/soft-serve/pkg/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// RunServer starts the gRPC management server
func RunServer(ctx context.Context, be *backend.Backend) error {
	cfg := config.FromContext(ctx)
	logger := log.FromContext(ctx).WithPrefix("grpc")

	if !cfg.GRPC.Enabled {
		logger.Debug("gRPC server is disabled")
		return nil
	}

	lis, err := net.Listen("tcp", cfg.GRPC.ListenAddr)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	RegisterGitServerManagementServer(grpcServer, NewServer(ctx, be))

	// Register reflection service for grpcurl and other tools
	reflection.Register(grpcServer)

	logger.Info("Starting gRPC management server", "addr", cfg.GRPC.ListenAddr)

	go func() {
		<-ctx.Done()
		logger.Info("Stopping gRPC server")
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(lis); err != nil {
		return err
	}

	return nil
}
