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

package fnmap

import (
	"context"
	"fmt"
	"sync"

	"github.com/fnrunner/fnruntime/pkg/fnproxy/clients"
	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	ctrlcfgv1alpha1 "github.com/fnrunner/fnsyntax/apis/controllerconfig/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Initializer func() Function

type FuncMap interface {
	Register(fnType ctrlcfgv1alpha1.FunctionType, initFn Initializer)
	Run(ctx context.Context, vertexContext *rtdag.VertexContext, i input.Input) (output.Output, error)
}

type Config struct {
	Name           string
	Namespace      string
	RootVertexName string
	Client         client.Client
	Output         output.Output
	Result         result.Result
	FnClients      *clients.Clients
}

func New(c *Config) FuncMap {
	return &fnMap{
		cfg:   c,
		funcs: map[ctrlcfgv1alpha1.FunctionType]Initializer{},
	}
}

type fnMap struct {
	cfg   *Config
	m     sync.RWMutex
	funcs map[ctrlcfgv1alpha1.FunctionType]Initializer
}

func (r *fnMap) Register(fnType ctrlcfgv1alpha1.FunctionType, initFn Initializer) {
	r.m.Lock()
	defer r.m.Unlock()
	r.funcs[fnType] = initFn
}

func (r *fnMap) Run(ctx context.Context, vertexContext *rtdag.VertexContext, i input.Input) (output.Output, error) {
	r.m.RLock()
	initializer, ok := r.funcs[vertexContext.Function.Type]
	r.m.RUnlock()
	//fmt.Printf("fnmap run %s, type: %s\n", vertexContext.VertexName, string(vertexContext.Function.Type))
	if !ok {
		return nil, fmt.Errorf("function not registered, got: %s", string(vertexContext.Function.Type))
	}
	// initialize the function
	fn := initializer()
	// initialize the runtime info
	switch vertexContext.Function.Type {
	case ctrlcfgv1alpha1.BlockType:
		fn.WithOutput(r.cfg.Output)
		fn.WithResult(r.cfg.Result)
		fn.WithFnMap(r)
	case ctrlcfgv1alpha1.QueryType:
		fn.WithClient(r.cfg.Client)
	case ctrlcfgv1alpha1.ContainerType, ctrlcfgv1alpha1.WasmType:
		fn.WithNameAndNamespace(r.cfg.Name, r.cfg.Namespace)
		fn.WithRootVertexName(r.cfg.RootVertexName)
		fn.WithFnClients(r.cfg.FnClients)
	}
	// run the function
	return fn.Run(ctx, vertexContext, i)
}
