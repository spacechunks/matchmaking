package functional

import (
	"context"
	"fmt"
	"net"

	"github.com/google/uuid"
	chunkv1alpha1 "github.com/spacechunks/explorer/api/chunk/v1alpha1"
	instancev1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type FakeControlPlane struct {
	chunkv1alpha1.UnimplementedChunkServiceServer
	instancev1alpha1.UnimplementedInstanceServiceServer

	flavors    map[string]*chunkv1alpha1.Flavor
	listenAddr string
}

func (f FakeControlPlane) Run(ctx context.Context) error {
	grpcServer := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))

	chunkv1alpha1.RegisterChunkServiceServer(grpcServer, f)
	instancev1alpha1.RegisterInstanceServiceServer(grpcServer, f)

	lis, err := net.Listen("tcp", f.listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	cancelCtx, cancel := context.WithCancel(ctx)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			fmt.Println("error serving grpc:", err)
			cancel()
		}
	}()

	<-cancelCtx.Done()
	grpcServer.GracefulStop()
	return nil
}

func (f FakeControlPlane) GetFlavor(ctx context.Context, request *chunkv1alpha1.GetFlavorRequest) (*chunkv1alpha1.GetFlavorResponse, error) {
	flavor, ok := f.flavors[request.Id]
	if !ok {
		return nil, status.Error(codes.NotFound, "not found")
	}

	return &chunkv1alpha1.GetFlavorResponse{
		Flavor: flavor,
	}, nil
}

func (f FakeControlPlane) RunFlavorVersion(ctx context.Context, request *instancev1alpha1.RunFlavorVersionRequest) (*instancev1alpha1.RunFlavorVersionResponse, error) {
	return &instancev1alpha1.RunFlavorVersionResponse{
		Instance: &instancev1alpha1.Instance{
			Id: uuid.NewString(),
		},
	}, nil
}
