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
	"fmt"
	"sync"

	"github.com/fnrunner/fnruntime/pkg/fnproxy/clients"
	"github.com/fnrunner/fnruntime/pkg/exec/exechandler"
	"github.com/fnrunner/fnruntime/pkg/exec/fnmap"
	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	"github.com/fnrunner/fnutils/pkg/executor"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewBlockFn() fnmap.Function {
	l := ctrl.Log.WithName("block fn")
	r := &block{
		l: l,
	}

	r.fec = &fnExecConfig{
		executeRange:  false,
		executeSingle: true,
		// execution functions
		filterInputFn: r.filterInput,
		runFn:         r.run,
		// result functions
		initOutputFn:     r.initOutput,
		recordOutputFn:   r.recordOutput,
		getFinalResultFn: r.getFinalResult,
		l:                l,
	}
	return r
}

type block struct {
	// fec exec config
	fec *fnExecConfig
	// init config
	controllerName string
	curOutputs output.Output // this is the current output list
	curResults result.Result
	fnMap      fnmap.FuncMap
	// runtime config
	d rtdag.RuntimeDAG
	// result, output
	m      sync.RWMutex
	output []any
	// logging
	l logr.Logger
}

func (r *block) Init(opts ...fnmap.FunctionOption) {
	for _, o := range opts {
		o(r)
	}
}

func (r *block) WithOutput(output output.Output) {
	r.curOutputs = output
}

func (r *block) WithResult(result result.Result) {
	r.curResults = result
}

func (r *block) WithNameAndNamespace(name, namespace string) {}

func (r *block) WithRootVertexName(name string) {}

func (r *block) WithClient(client client.Client) {}

func (r *block) WithFnMap(fnMap fnmap.FuncMap) {
	r.fnMap = fnMap
}

func (r *block) WithFnClients(fnc *clients.Clients) {}

func (r *block) WithControllerName(name string) {
	r.controllerName = name
}

func (r *block) Run(ctx context.Context, vertexContext *rtdag.VertexContext, i input.Input) (output.Output, error) {
	r.l.Info("run", "vertexName", vertexContext.VertexName, "input", i.Get())
	// Here we prepare the input we get from the runtime
	// e.g. DAG, outputs/outputInfo (internal/GVK/etc), fnConfig parameters, etc etc
	r.d = vertexContext.BlockDAG

	// execute to function
	return r.fec.exec(ctx, vertexContext.Function, i)
}

func (r *block) initOutput(numItems int) {
	r.output = make([]any, 0, numItems)
}

func (r *block) recordOutput(o any) {
	r.m.Lock()
	defer r.m.Unlock()
	r.output = append(r.output, o)
}

func (r *block) getFinalResult() (output.Output, error) {
	if len(r.output) == 0 {
		return output.New(), nil
	}
	return output.New(), nil
}

func (r *block) filterInput(i input.Input) input.Input { return i }

func (r *block) run(ctx context.Context, i input.Input) (any, error) {
	// check if the dag is initialized
	if r.d == nil {
		err := fmt.Errorf("expecting an initialized dag, got: %T", r.d)
		r.l.Error(err, "dag not initialized")
		return nil, err
	}

	// debug
	//r.d.PrintVertices()
	r.l.Info("rootVertex", "name", r.d.GetRootVertex())
	//fmt.Printf("block root Vertex: %s\n", r.d.GetRootVertex())

	rootVertexName := r.d.GetRootVertex()

	// initialize the handler
	h := exechandler.New(&exechandler.Config{
		Name:   rootVertexName,
		Type:   result.ExecBlockType,
		DAG:    r.d,
		FnMap:  r.fnMap,
		Output: r.curOutputs,
		Result: r.curResults,
	})

	e := executor.New(r.d, &executor.Config{
		Name:               rootVertexName,
		From:               rootVertexName,
		VertexFuntionRunFn: h.FunctionRun,
		ExecPostRunFn:      h.RecordFinalResult,
	})
	e.Run(ctx)

	return nil, nil

}
