/*
Copyright 2023 Nokia.

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

package exechandler

import (
	"context"

	"github.com/fnrunner/fnproto/pkg/executor/executorpb"
	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
)

func (r *subServer) ExecuteFuntion(ctx context.Context, req *executorpb.ExecuteFunctionRequest) (*executorpb.ExecuteFunctionResponse, error) {
	r.l.Info("execute function", "image", req.Image, "controllerName", req.GetController())

	imageStore := r.ctrlStore.GetImageStore(req.GetController())
	if imageStore == nil {
		return &executorpb.ExecuteFunctionResponse{}, ErrClientNotready
	}

	execclient := imageStore.GetFnClient(fnrunv1alpha1.Image{Name: req.GetImage(), Kind: fnrunv1alpha1.ImageKindFunction})
	if execclient == nil {
		return &executorpb.ExecuteFunctionResponse{}, ErrClientNotready
	}
	r.l.Info("execute function", "client config", execclient.GetConfig())
	resp, err := execclient.Get().ExecuteFunction(ctx, req)
	if err != nil {
		r.l.Info("cannot execute function", "err", err)
	}
	return resp, err
}
