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

	"github.com/fnrunner/fnproto/pkg/executor/executorpb"
)

func (r *GrpcServer) ExecuteFunction(ctx context.Context, req *executorpb.ExecuteFunctionRequest) (*executorpb.ExecuteFunctionResponse, error) {
	r.l.Info("execute fn", "req", req)
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	err := r.acquireSem(ctx)
	if err != nil {
		return nil, err
	}
	defer r.sem.Release(1)
	return r.execHandler(ctx, req)
}
