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

package functions

import (
	"context"

	"github.com/fnrunner/fnruntime/internal/fnproxy/clients"
	"github.com/fnrunner/fnruntime/pkg/exec/fnmap"
	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewRootFn() fnmap.Function {
	l := ctrl.Log.WithName("root fn")
	r := &root{
		l: l,
	}

	r.fec = &fnExecConfig{
		executeRange:  false,
		executeSingle: false,
		// execution functions
		//filterInputFn: r.filterInput,
		// result functions
		getFinalResultFn: r.getFinalResult,
		l:                l,
	}
	return r
}

type root struct {
	// fec exec config
	fec *fnExecConfig
	// logging
	l logr.Logger
}

func (r *root) Init(opts ...fnmap.FunctionOption) {
	for _, o := range opts {
		o(r)
	}
}

func (r *root) WithOutput(output output.Output) {}

func (r *root) WithResult(result result.Result) {}

func (r *root) WithNameAndNamespace(name, namespace string) {}

func (r *root) WithRootVertexName(name string) {}

func (r *root) WithClient(client client.Client) {}

func (r *root) WithFnMap(fnMap fnmap.FuncMap) {}

func (r *root) WithFnClients(fnc *clients.Clients) {}

func (r *root) Run(ctx context.Context, vertexContext *rtdag.VertexContext, i input.Input) (output.Output, error) {
	// Here we prepare the input we get from the runtime
	// e.g. DAG, outputs/outputInfo (internal/GVK/etc), fnConfig parameters, etc etc
	// execute the function
	r.l.Info("run", "vertexName", vertexContext.VertexName, "input", i.Get())
	return r.fec.exec(ctx, vertexContext.Function, i)
}

func (r *root) getFinalResult() (output.Output, error) {
	return output.New(), nil
}

//func (r *root) filterInput(i input.Input) input.Input {return i}
