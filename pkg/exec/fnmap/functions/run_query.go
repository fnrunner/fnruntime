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

	"github.com/fnrunner/fnruntime/pkg/fnproxy/clients"
	"github.com/fnrunner/fnruntime/pkg/exec/fnmap"
	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	"github.com/fnrunner/fnutils/pkg/meta"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func NewQueryFn() fnmap.Function {
	l := ctrl.Log.WithName("query fn")
	r := &query{
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

type query struct {
	// fec exec config
	fec *fnExecConfig
	// init config
	client client.Client
	// runtime config
	outputs  output.Output
	resource runtime.RawExtension
	selector *metav1.LabelSelector
	// output, output
	output any
	// logging
	l logr.Logger
}

func (r *query) Init(opts ...fnmap.FunctionOption) {
	for _, o := range opts {
		o(r)
	}
}

func (r *query) WithOutput(output output.Output) {}

func (r *query) WithResult(result result.Result) {}

func (r *query) WithNameAndNamespace(name, namespace string) {}

func (r *query) WithRootVertexName(name string) {}

func (r *query) WithClient(client client.Client) {
	r.client = client
}

func (r *query) WithFnMap(fnMap fnmap.FuncMap) {}

func (r *query) WithFnClients(fnc *clients.Clients) {}

func (r *query) Run(ctx context.Context, vertexContext *rtdag.VertexContext, i input.Input) (output.Output, error) {
	r.l.Info("run", "vertexName", vertexContext.VertexName, "input", i.Get(), "resource", vertexContext.Function.Input.Resource)
	// Here we prepare the input we get from the runtime
	// e.g. DAG, outputs/outputInfo (internal/GVK/etc), fnConfig parameters, etc etc
	r.outputs = vertexContext.Outputs
	r.resource = vertexContext.Function.Input.Resource
	//r.selector = vertexContext.Function.Input.Selector

	// execute to function
	return r.fec.exec(ctx, vertexContext.Function, i)
}

func (r *query) initOutput(numItems int) {}

func (r *query) recordOutput(o any) {
	r.output = o
}

func (r *query) getFinalResult() (output.Output, error) {
	o := output.New()
	for varName, v := range r.outputs.Get() {
		oi, ok := v.(*output.OutputInfo)
		if !ok {
			err := fmt.Errorf("expecting outputInfo, got %T", v)
			r.l.Error(err, "cannot record result")
			return o, err
		}
		o.AddEntry(varName, &output.OutputInfo{
			Internal: oi.Internal,
			GVK:      oi.GVK,
			Data:     r.output,
		})
	}
	//o.Print()
	return o, nil
}

func (r *query) filterInput(i input.Input) input.Input { return i }

func (r *query) run(ctx context.Context, i input.Input) (any, error) {
	gvk, err := meta.GetGVKFromRuntimeRawExtension(r.resource)
	if err != nil {
		r.l.Error(err, "cannot get GVK")
		return nil, err
	}
	r.l.Info("query run", "gvk", gvk)

	opts := []client.ListOption{}
	if r.selector != nil {
		// TODO namespace
		//opts = append(opts, client.InNamespace("x"))
		opts = append(opts, client.MatchingLabels(r.selector.MatchLabels))
	}

	o := meta.GetUnstructuredListFromGVK(gvk)
	if err := r.client.List(ctx, o, opts...); err != nil {
		r.l.Error(err, "cannot list gvk", "gvk", gvk, "options", opts)
		return nil, err
	}

	rj := make([]interface{}, 0, len(o.Items))
	for _, v := range o.Items {
		b, err := yaml.Marshal(v.UnstructuredContent())
		if err != nil {
			r.l.Error(err, "cannot marshal data")
			return nil, err
		}

		vrj := map[string]interface{}{}
		if err := yaml.Unmarshal(b, &vrj); err != nil {
			r.l.Error(err, "cannot unmarshal data")
			return nil, err
		}
		rj = append(rj, vrj)
	}

	return rj, nil
}
