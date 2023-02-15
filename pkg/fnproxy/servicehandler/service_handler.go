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

package servicehandler

import (
	"context"

	"github.com/fnrunner/fnproto/pkg/service/servicepb"
	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

func (r *subServer) ApplyResource(ctx context.Context, req *servicepb.FunctionServiceRequest) (*servicepb.FunctionServiceResponse, error) {
	r.l.Info("service apply", "image", req.Image, "controllerName", req.GetController())

	imageStore := r.ctrlStore.GetImageStore(req.GetController())
	if imageStore == nil {
		return &servicepb.FunctionServiceResponse{}, ErrClientNotready
	}

	svcclient := imageStore.GetSvcClient(fnrunv1alpha1.Image{Name: req.GetImage(), Kind: fnrunv1alpha1.ImageKindService})
	if svcclient == nil {
		return &servicepb.FunctionServiceResponse{}, ErrClientNotready
	}
	return svcclient.Get().ApplyResource(ctx, req)
}

func (r *subServer) DeleteResource(ctx context.Context, req *servicepb.FunctionServiceRequest) (*emptypb.Empty, error) {
	r.l.Info("service delete", "image", req.Image, "controllerName", req.GetController())

	imageStore := r.ctrlStore.GetImageStore(req.GetController())
	if imageStore == nil {
		return &emptypb.Empty{}, ErrClientNotready
	}

	svcclient := imageStore.GetSvcClient(fnrunv1alpha1.Image{Name: req.GetImage(), Kind: fnrunv1alpha1.ImageKindService})
	if svcclient == nil {
		return &emptypb.Empty{}, ErrClientNotready
	}
	return svcclient.Get().DeleteResource(ctx, req)
}
