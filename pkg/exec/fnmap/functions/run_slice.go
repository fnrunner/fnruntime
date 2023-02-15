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
	"errors"
	"fmt"
	"sync"

	"github.com/fnrunner/fnruntime/pkg/fnproxy/clients"
	"github.com/fnrunner/fnruntime/pkg/exec/fnmap"
	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	"github.com/go-logr/logr"
	"github.com/itchyny/gojq"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewSliceFn() fnmap.Function {
	l := ctrl.Log.WithName("slice fn")
	r := &slice{
		l: l,
	}

	r.fec = &fnExecConfig{
		executeRange:  true,
		executeSingle: false,
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

type slice struct {
	// fec exec config
	fec *fnExecConfig
	// init config
	controllerName string
	// runtime config
	outputs output.Output
	value   string
	// result, output
	m      sync.RWMutex
	output []any
	// logging
	l logr.Logger
}

func (r *slice) Init(opts ...fnmap.FunctionOption) {
	for _, o := range opts {
		o(r)
	}
}

func (r *slice) WithOutput(output output.Output) {}

func (r *slice) WithResult(result result.Result) {}

func (r *slice) WithNameAndNamespace(name, namespace string) {}

func (r *slice) WithRootVertexName(name string) {}

func (r *slice) WithClient(client client.Client) {}

func (r *slice) WithFnMap(fnMap fnmap.FuncMap) {}

func (r *slice) WithFnClients(fnc *clients.Clients) {}

func (r *slice) WithControllerName(name string) {
	r.controllerName = name
}

func (r *slice) Run(ctx context.Context, vertexContext *rtdag.VertexContext, i input.Input) (output.Output, error) {
	r.l.Info("run", "vertexName", vertexContext.VertexName, "input", i.Get(), "expression", r.value)
	// Here we prepare the input we get from the runtime
	// e.g. DAG, outputs/outputInfo (internal/GVK/etc), fnConfig parameters, etc etc
	r.outputs = vertexContext.Outputs
	r.value = vertexContext.Function.Input.Value

	// execute the function
	return r.fec.exec(ctx, vertexContext.Function, i)
}

func (r *slice) initOutput(numItems int) {
	r.output = make([]any, 0, numItems)
}

func (r *slice) recordOutput(o any) {
	r.m.Lock()
	defer r.m.Unlock()
	r.output = append(r.output, o)
}

func (r *slice) getFinalResult() (output.Output, error) {
	o := output.New()
	for varName, v := range r.outputs.Get() {
		oi, ok := v.(*output.OutputInfo)
		if !ok {
			err := fmt.Errorf("expecting outputInfo, got %T", v)
			r.l.Error(err, "cannot get result")
			return o, err
		}
		o.AddEntry(varName, &output.OutputInfo{
			Internal: oi.Internal,
			GVK:      oi.GVK,
			Data:     r.output,
		})
	}
	return o, nil
}

func (r *slice) filterInput(i input.Input) input.Input { return i }

func (r *slice) run(ctx context.Context, i input.Input) (any, error) {
	if r.value == "" {
		err := errors.New("missing input value")
		r.l.Error(err, "wrong input value")
		return nil, err
	}
	varNames := make([]string, 0, i.Length())
	varValues := make([]any, 0, i.Length())
	for name, v := range i.Get() {
		varNames = append(varNames, "$"+name)
		varValues = append(varValues, v)
	}
	//fmt.Printf("buildSliceItem varNames: %v, varValues: %v\n", varNames, varValues)
	//fmt.Printf("buildSliceItem exp: %s\n", r.value)
	r.l.Info("buildSliceItem", "varNames", varNames, "varValues", varValues, "expression", r.value)

	q, err := gojq.Parse(r.value)
	if err != nil {
		r.l.Error(err, "cannot parse jq", "expression", r.value)
		return nil, err
	}
	code, err := gojq.Compile(q, gojq.WithVariables(varNames))
	if err != nil {
		r.l.Error(err, "cannot conpile jq", "varNames", varNames)
		return nil, err
	}

	iter := code.Run(nil, varValues...)
	v, ok := iter.Next()
	if !ok {
		err := errors.New("no value")
		r.l.Error(err, "wrong result")
		return nil, err
	}
	if err, ok := v.(error); ok {
		if err != nil {
			r.l.Error(err, "run jq error")
			return nil, err
		}
	}
	r.l.Info("buildSliceItem", "value", v)
	return v, nil
}
