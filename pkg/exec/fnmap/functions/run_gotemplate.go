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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"text/template"

	"github.com/fnrunner/fnruntime/pkg/fnproxy/clients"
	"github.com/fnrunner/fnruntime/pkg/exec/fnmap"
	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewGTFn() fnmap.Function {
	l := ctrl.Log.WithName("gotemplate fn")
	r := &gt{
		l: l,
	}

	r.fec = &fnExecConfig{
		executeRange:  true,
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

type gt struct {
	// fec exec config
	fec *fnExecConfig
	// init config
	controllerName string
	// runtime config
	outputs  output.Output
	template string
	// result, output
	m      sync.RWMutex
	output []any
	// logging
	l logr.Logger
}

func (r *gt) Init(opts ...fnmap.FunctionOption) {
	for _, o := range opts {
		o(r)
	}
}

func (r *gt) WithOutput(output output.Output) {}

func (r *gt) WithResult(result result.Result) {}

func (r *gt) WithNameAndNamespace(name, namespace string) {}

func (r *gt) WithRootVertexName(name string) {}

func (r *gt) WithClient(client client.Client) {}

func (r *gt) WithFnMap(fnMap fnmap.FuncMap) {}

func (r *gt) WithFnClients(fnc *clients.Clients) {}

func (r *gt) WithControllerName(name string) {
	r.controllerName = name
}

func (r *gt) Run(ctx context.Context, vertexContext *rtdag.VertexContext, i input.Input) (output.Output, error) {
	r.l.Info("run", "vertexName", vertexContext.VertexName, "input", i.Get(), "resource", vertexContext.Function.Input.Resource.Raw)

	// Here we prepare the input we get from the runtime
	// e.g. DAG, outputs/outputInfo (internal/GVK/etc), fnConfig parameters, etc etc
	r.outputs = vertexContext.Outputs
	if len(vertexContext.Function.Input.Resource.Raw) != 0 {
		r.template = string(vertexContext.Function.Input.Resource.Raw)
	} else {
		r.template = vertexContext.Function.Input.Template
	}

	// execute the function
	return r.fec.exec(ctx, vertexContext.Function, i)
}

func (r *gt) initOutput(numItems int) {
	r.output = make([]any, 0, numItems)
}

func (r *gt) recordOutput(o any) {
	r.m.Lock()
	defer r.m.Unlock()
	r.output = append(r.output, o)
}

func (r *gt) getFinalResult() (output.Output, error) {
	o := output.New()
	for varName, v := range r.outputs.Get() {
		oi, ok := v.(*output.OutputInfo)
		if !ok {
			err := fmt.Errorf("expecting outputInfo, got %T", v)
			r.l.Error(err, "got wrong type")
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

func (r *gt) filterInput(i input.Input) input.Input { return i }

func (r *gt) run(ctx context.Context, i input.Input) (any, error) {
	if r.template == "" {
		err := errors.New("missing template")
		r.l.Error(err, "cannot run gotemplate without a template")
		return nil, err
	}
	result := new(bytes.Buffer)
	// TODO: add template custom functions
	tpl, err := template.New("default").Option("missingkey=zero").Parse(r.template)
	if err != nil {
		r.l.Error(err, "cannot parse template")
		return nil, err
	}
	r.l.Info("run", "input", i.Get())
	err = tpl.Execute(result, i.Get())
	if err != nil {
		return nil, err
	}
	var x any
	err = json.Unmarshal(result.Bytes(), &x)
	r.l.Info("run", "output", x)
	return x, err
}
