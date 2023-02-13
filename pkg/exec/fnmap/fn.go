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

	"github.com/fnrunner/fnruntime/pkg/fnproxy/clients"
	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Function interface {
	// Init initializes the device
	Init(...FunctionOption)
	WithOutput(output output.Output)
	WithResult(result result.Result)
	WithNameAndNamespace(name, namespace string)
	WithClient(client client.Client)
	WithFnMap(fnMap FuncMap)
	WithRootVertexName(name string)
	WithFnClients(*clients.Clients)
	Run(ctx context.Context, vertexContext *rtdag.VertexContext, i input.Input) (output.Output, error)
}

type FunctionOption func(Function)

func WithOutput(output output.Output) FunctionOption {
	return func(r Function) {
		r.WithOutput(output)
	}
}

func WithResult(result result.Result) FunctionOption {
	return func(r Function) {
		r.WithResult(result)
	}
}

func WithNameAndNamespace(name, namespace string) FunctionOption {
	return func(r Function) {
		r.WithNameAndNamespace(name, namespace)
	}
}

func WithClient(client client.Client) FunctionOption {
	return func(r Function) {
		r.WithClient(client)
	}
}

func WithFnMap(fnMap FuncMap) FunctionOption {
	return func(r Function) {
		r.WithFnMap(fnMap)
	}
}

func WithRootVertexName(name string) FunctionOption {
	return func(r Function) {
		r.WithRootVertexName(name)
	}
}

func WithFnClients(fnc *clients.Clients) FunctionOption {
	return func(r Function) {
		r.WithFnClients(fnc)
	}
}
