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

package exechandler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fnrunner/fnruntime/pkg/exec/fnmap"
	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	ctrlcfgv1alpha1 "github.com/fnrunner/fnsyntax/apis/controllerconfig/v1alpha1"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ExecHandler interface {
	FunctionRun(ctx context.Context, vertexName string, vertexContext any) bool
	RecordFinalResult(start, finish time.Time, success bool)
}

type Config struct {
	Name   string
	Type   result.ExecType
	DAG    rtdag.RuntimeDAG
	FnMap  fnmap.FuncMap
	Output output.Output
	Result result.Result
}

func New(c *Config) ExecHandler {
	return &execHandler{
		cfg: c,
		l:   ctrl.Log.WithName("execHandler"),
	}
}

type execHandler struct {
	cfg *Config
	l   logr.Logger
}

func (r *execHandler) FunctionRun(ctx context.Context, vertexName string, vertexContext any) bool {
	start := time.Now()
	success := true
	reason := ""
	rootVertexName := r.cfg.DAG.GetRootVertex()

	r.l.WithValues("execName", rootVertexName, "vertexName", vertexName)

	vc, ok := vertexContext.(*rtdag.VertexContext)
	if !ok {
		err := fmt.Errorf("expecting *rtdag.VertexContext, got %T", vertexContext)
		r.l.Error(err, "wrong context input")
		r.cfg.Result.Add(&result.ResultInfo{
			Type:       r.cfg.Type,
			ExecName:   r.cfg.Name,
			VertexName: vertexName,
			StartTime:  start,
			EndTime:    time.Now(),
			Success:    false,
			Reason:     err.Error(),
		})
	}

	// Gather the input based on the function type
	i := input.New()
	switch vc.Function.Type {
	case ctrlcfgv1alpha1.RootType:
		// this is a dummy function, input is not relevant
	case ctrlcfgv1alpha1.ContainerType, ctrlcfgv1alpha1.WasmType:
		i.AddEntry(rootVertexName, r.cfg.Output.GetData(rootVertexName))
		for _, ref := range vc.References {
			i.AddEntry(ref, r.cfg.Output.GetData(ref))
		}
	default:
		r.l.Info("prepare input", "references", vc.References)
		//fmt.Printf("execContext execName %s vertexName: %s references: %v\n", rootVertexName, vc.VertexName, vc.References)
		for _, ref := range vc.References {
			i.AddEntry(ref, r.cfg.Output.GetData(ref))
		}
	}
	//i.Print(vertexName)

	o, err := r.cfg.FnMap.Run(ctx, vc, i)
	if err != nil {
		if !errors.Is(err, ErrConditionFalse) {
			success = false
		}
		reason = err.Error()
	}

	finished := time.Now()

	r.cfg.Output.Add(o)

	r.cfg.Result.Add(&result.ResultInfo{
		Type:       r.cfg.Type,
		ExecName:   r.cfg.Name,
		VertexName: vertexName,
		StartTime:  start,
		EndTime:    finished,
		Input:      i,
		Output:     o,
		Success:    success,
		Reason:     reason,
	})
	return success
}

func (r *execHandler) RecordFinalResult(start, finish time.Time, success bool) {
	r.cfg.Result.Add(&result.ResultInfo{
		Type:       result.ExecRootType,
		ExecName:   r.cfg.DAG.GetRootVertex(),
		VertexName: "total",
		StartTime:  start,
		EndTime:    finish,
		Success:    success,
	})
}
