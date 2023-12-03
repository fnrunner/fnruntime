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
	"strings"

	"github.com/fnrunner/fnruntime/pkg/exec/exechandler"
	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	ctrlcfgv1alpha1 "github.com/fnrunner/fnsyntax/apis/controllerconfig/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/itchyny/gojq"
)

type initOutputFn func(numItems int)
type recordOutputFn func(any)
type getFinalResultFn func() (output.Output, error)
type filterInputFn func(input.Input) input.Input
type runFn func(context.Context, input.Input) (any, error)

type fnExecConfig struct {
	executeRange  bool
	executeSingle bool
	// execution functions
	//prepareInputFn prepareInputFn
	filterInputFn filterInputFn
	runFn         runFn
	// result functions
	initOutputFn     initOutputFn
	recordOutputFn   recordOutputFn
	getFinalResultFn getFinalResultFn
	// logging
	l logr.Logger
}

func (r *fnExecConfig) exec(ctx context.Context, fnconfig ctrlcfgv1alpha1.Function, i input.Input) (output.Output, error) {
	var items []*item
	var isRange bool
	var ok bool
	var err error
	if fnconfig.HasBlock() {
		r.l.Info("execute block")
		if fnconfig.Block.Range != nil {
			r.l.Info("execute range", "value", fnconfig.Block.Range.Value)
			items, err = runRange(fnconfig.Block.Range.Value, i)
			if err != nil {
				r.l.Error(err, "cannot run range")
				return nil, err
			}
			r.l.Info("range", "items", items)
			isRange = true
		}
		if fnconfig.Block.Condition != nil {
			r.l.Info("execute condition", "expression", fnconfig.Block.Condition.Expression)
			if exp := fnconfig.Block.Condition.Expression; exp != "" {
				ok, err = runCondition(exp, i)
				if err != nil {
					r.l.Error(err, "cannot run condition")
					return nil, err
				}
				if !ok {
					return output.New(), exechandler.ErrConditionFalse // error to be ignored, condition false, so we dont have to run
				}
			}
			if fnconfig.Block.Condition.Block.Range != nil {
				r.l.Info("execute range in condition", "value", fnconfig.Block.Condition.Block.Range.Value)
				items, err = runRange(fnconfig.Block.Condition.Block.Range.Value, i)
				if err != nil {
					r.l.Error(err, "cannot run range in condition")
					return nil, err
				}
				isRange = true
			}
		}
	}
	numItems := len(items)
	if numItems == 0 && isRange {
		r.initOutputFn(0)
		return nil, nil // no entries in the range, so we are done
	}
	if numItems > 0 && isRange {
		r.initOutputFn(numItems)
		for n, item := range items {
			//fmt.Printf("range items: n: %d, item %#v\n", n, item)
			// this is a protection to ensure we dont use the nil result in a range
			if item.val != nil {
				i.AddEntry("VALUE", item.val)
				i.AddEntry("KEY", fmt.Sprint(n))
				i.AddEntry("INDEX", n)

				// resolve the local vars using jq and add them to the input
				if err := resolveLocalVars(fnconfig, i); err != nil {
					return nil, err
				}

				//i.Print("range")

				if r.executeRange {
					//extraInput := fec.prepareInputFn(fnconfig)
					fi := r.filterInputFn(i)
					x, err := r.runFn(ctx, fi)
					if err != nil {
						return nil, err
					}

					// TODO add hook for service resolution
					r.recordOutputFn(x)
				}
			}
		}
	}
	if r.executeSingle {
		r.l.Info("execute single")
		r.initOutputFn(1)
		// resolve the local vars using jq and add them to the input
		if err := resolveLocalVars(fnconfig, i); err != nil {
			return nil, err
		}
		//extraInput := fec.prepareInputFn(fnconfig)
		fi := r.filterInputFn(i)
		x, err := r.runFn(ctx, fi)
		if err != nil {
			return nil, err
		}
		// TODO add hook for service resolution
		r.recordOutputFn(x)
	}
	return r.getFinalResultFn()
}

type item struct {
	//key string
	val any
}

func runRange(exp string, i input.Input) ([]*item, error) {
	varNames := make([]string, 0, i.Length())
	varValues := make([]any, 0, i.Length())
	for name, v := range i.Get() {
		varNames = append(varNames, "$"+name)
		varValues = append(varValues, v)
		//fmt.Printf("runRange variables: name: %s, value: %v\n", name, v)
	}
	//fmt.Printf("runRange varNames: %v, varValues: %v\n", varNames, varValues)
	//fmt.Printf("runRange exp: %s\n", exp)

	q, err := gojq.Parse(exp)
	if err != nil {
		return nil, err
	}
	code, err := gojq.Compile(q, gojq.WithVariables(varNames))
	if err != nil {
		return nil, err
	}
	result := make([]*item, 0)
	iter := code.Run(nil, varValues...)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if err != nil {
				//fmt.Printf("runJQ err: %v\n", err)
				if strings.Contains(err.Error(), "cannot iterate over: null") {
					return result, nil
				}
				return nil, err
			}
		}
		if v == nil {
			continue
		}
		//fmt.Printf("runRange result item: %#v\n", v)
		result = append(result, &item{val: v})
	}

	return result, nil
}

func runCondition(exp string, i input.Input) (bool, error) {
	varNames := make([]string, 0, i.Length())
	varValues := make([]any, 0, i.Length())
	for name, v := range i.Get() {
		varNames = append(varNames, "$"+name)
		varValues = append(varValues, v)
	}

	q, err := gojq.Parse(exp)
	if err != nil {
		return false, err
	}
	code, err := gojq.Compile(q, gojq.WithVariables(varNames))
	if err != nil {
		return false, err
	}
	iter := code.Run(nil, varValues...)

	v, ok := iter.Next()
	if !ok {
		return false, errors.New("not result")
	}

	if err, ok := v.(error); ok {
		if err != nil {
			return false, err
		}
	}
	if r, ok := v.(bool); ok {
		return r, nil
	}
	return false, fmt.Errorf("unexpected result type, want bool got %T", v)
}

func resolveLocalVars(fnconfig ctrlcfgv1alpha1.Function, i input.Input) error {
	if fnconfig.Vars != nil {
		for varName, expression := range fnconfig.Vars {
			// We are lazy and provide all reference input to JQ
			// the below aproach could be a more optimal solution
			// but for now we keep it simple

			//	localVarRefs := make(map[string]any)
			//	rfs := ccsyntax.NewReferences()
			//	refs := rfs.GetReferences(expression)
			//	for _, ref := range refs {
			//		localVarRefs[ref.Value] = input[ref.Value]
			//	}
			//fmt.Printf("resolveLocalVars varname: %s expression %s\n", varName, expression)

			v, err := runJQ(expression, i)
			if err != nil {
				return err
			}
			//fmt.Printf("resolveLocalVars varname: %s jq %#v\n", varName, v)

			i.AddEntry(varName, v)
		}
	}
	return nil
}
