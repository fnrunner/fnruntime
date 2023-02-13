/*
Copyright 2022 Nokia.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// Check implements `service Health`.
func (r *GrpcServer) Check(ctx context.Context, in *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	err := r.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer r.sem.Release(1)

	if r.checkHandler != nil {
		return r.checkHandler(ctx, in)
	}

	return &healthpb.HealthCheckResponse{}, nil
}

// Watch implements `service Health`.
func (r *GrpcServer) Watch(in *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	err := r.acquireSem(stream.Context())
	if err != nil {
		return err
	}
	defer r.sem.Release(1)

	if r.watchHandler != nil {
		return r.watchHandler(in, stream)
	}
	return status.Error(codes.Unimplemented, "")
}
