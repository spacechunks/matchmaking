/*
 A basic matchmaking service for the Chunk Explorer.
 Copyright (C) 2026 Yannic Rieger <oss@76k.io>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU Affero General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU Affero General Public License for more details.

 You should have received a copy of the GNU Affero General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

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

func (f FakeControlPlane) GetFlavor(
	ctx context.Context,
	req *chunkv1alpha1.GetFlavorRequest,
) (*chunkv1alpha1.GetFlavorResponse, error) {
	flavor, ok := f.flavors[req.Id]
	if !ok {
		return nil, status.Error(codes.NotFound, "not found")
	}

	return &chunkv1alpha1.GetFlavorResponse{
		Flavor: flavor,
	}, nil
}

func (f FakeControlPlane) RunFlavorVersion(
	_ context.Context,
	_ *instancev1alpha1.RunFlavorVersionRequest,
) (*instancev1alpha1.RunFlavorVersionResponse, error) {
	return &instancev1alpha1.RunFlavorVersionResponse{
		Instance: &instancev1alpha1.Instance{
			Id: uuid.NewString(),
		},
	}, nil
}
