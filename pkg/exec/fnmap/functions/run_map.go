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

	"github.com/fnrunner/fnruntime/internal/fnproxy/clients"
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

type mapInput struct {
	key   string
	value string
}

type mapOutput struct {
	key   string
	value any
}

func NewMapFn() fnmap.Function {
	l := ctrl.Log.WithName("map fn")
	r := &kv{
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

type kv struct {
	// fec exec config
	fec *fnExecConfig
	// init config
	// runtime config
	outputs output.Output
	key     string
	value   string
	// result, output
	m      sync.RWMutex
	output map[string]any
	// logging
	l logr.Logger
}

func (r *kv) Init(opts ...fnmap.FunctionOption) {
	for _, o := range opts {
		o(r)
	}
}

func (r *kv) WithOutput(output output.Output) {}

func (r *kv) WithResult(result result.Result) {}

func (r *kv) WithNameAndNamespace(name, namespace string) {}

func (r *kv) WithRootVertexName(name string) {}

func (r *kv) WithClient(client client.Client) {}

func (r *kv) WithFnMap(fnMap fnmap.FuncMap) {}

func (r *kv) WithFnClients(fnc *clients.Clients) {}

func (r *kv) Run(ctx context.Context, vertexContext *rtdag.VertexContext, i input.Input) (output.Output, error) {
	r.l.Info("run", "vertexName", vertexContext.VertexName, "input", i.Get(), "key", vertexContext.Function.Input.Key, "value", vertexContext.Function.Input.Value)

	// Here we prepare the input we get from the runtime
	// e.g. DAG, outputs/outputInfo (internal/GVK/etc), fnConfig parameters, etc etc
	r.outputs = vertexContext.Outputs
	r.key = vertexContext.Function.Input.Key
	r.value = vertexContext.Function.Input.Value

	// execute the function
	return r.fec.exec(ctx, vertexContext.Function, i)
}

func (r *kv) initOutput(numItems int) {
	r.output = make(map[string]any, numItems)
}

func (r *kv) recordOutput(o any) {
	out, ok := o.(*mapOutput)
	if !ok {
		err := fmt.Errorf("expecting mapOutput, got: %T", o)
		r.l.Error(err, "cannot record output")
		return
	}
	r.m.Lock()
	defer r.m.Unlock()
	r.output[out.key] = out.value
}

func (r *kv) getFinalResult() (output.Output, error) {
	o := output.New()
	for varName, v := range r.outputs.Get() {
		oi, ok := v.(*output.OutputInfo)
		if !ok {
			err := fmt.Errorf("expecting outputInfo, got %T", v)
			r.l.Error(err, "cannot record output")
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

func (r *kv) filterInput(i input.Input) input.Input { return i }

func (r *kv) run(ctx context.Context, i input.Input) (any, error) {
	kv := &mapInput{
		key:   r.key,
		value: r.value,
	}
	if kv.value == "" {
		err := errors.New("missing input value")
		r.l.Error(err, "cannot run wrong input", "kv", kv)
		return nil, err
	}
	if kv.key == "" {
		err := errors.New("missing input key")
		r.l.Error(err, "cannot run wrong input", "kv", kv)
		return nil, err
	}

	varNames := make([]string, 0, i.Length())
	varValues := make([]any, 0, i.Length())
	for name, v := range i.Get() {
		varNames = append(varNames, "$"+name)
		varValues = append(varValues, v)
	}

	r.l.Info("buildKV", "varNames", varNames, "varValues", varValues, "expression", kv.value)
	//fmt.Printf("buildKV varNames: %v, varValues: %v\n", varNames, varValues)
	//fmt.Printf("buildKV exp: %s\n", kv.value)

	valq, err := gojq.Parse(kv.value)
	if err != nil {
		r.l.Error(err, "cannot parse jq valq", "kv", kv)
		return nil, err
	}
	valC, err := gojq.Compile(valq, gojq.WithVariables(varNames))
	if err != nil {
		r.l.Error(err, "cannot compile jq valC", "varNames", varNames)
		return nil, err
	}
	keyq, err := gojq.Parse(kv.key)
	if err != nil {
		r.l.Error(err, "cannot buildKV keyq", "kv", kv)
		return nil, err
	}
	keyC, err := gojq.Compile(keyq, gojq.WithVariables(varNames))
	if err != nil {
		r.l.Error(err, "cannot buildKV keyC", "kv", kv)
		return nil, err
	}

	v, err := runJQOnce(valC, nil, varValues...)
	if err != nil {
		r.l.Error(err, "cannot buildKV runJQOnce valC")
		return nil, err
	}

	k, err := runJQOnce(keyC, nil, varValues...)
	if err != nil {
		r.l.Error(err, "cannot buildKV runJQOnce keyC")
		return nil, err
	}
	//fmt.Printf("buildKV k: %T %#v\n", k, k)
	r.l.Info("buildkv", "key", k)
	ks, ok := k.(string)
	if !ok {
		err := fmt.Errorf("unexpected key format: %T", k)
		r.l.Error(err, "wrong key formaat")
		return nil, err
	}
	return &mapOutput{key: ks, value: v}, nil
}
