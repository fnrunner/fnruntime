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

package exechandler

import (
	"context"

	"github.com/fnrunner/fnproto/pkg/executor/executorpb"
	"github.com/fnrunner/fnruntime/pkg/fnproxy/cache"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

type SubServer interface {
	ExecuteFuntion(ctx context.Context, in *executorpb.ExecuteFunctionRequest) (*executorpb.ExecuteFunctionResponse, error)
}

func New(c cache.Cache) SubServer {
	r := &subServer{
		l: ctrl.Log.WithName("subserverExec"),
		c: c,
	}
	return r
}

type subServer struct {
	l logr.Logger
	c cache.Cache
}
