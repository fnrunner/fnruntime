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

package result

import (
	"fmt"
	"time"

	"github.com/fnrunner/fnruntime/pkg/exec/input"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnutils/pkg/slice"
)

type Result interface {
	slice.Slice
	Print()
}

type ExecType string

const (
	ExecRootType  ExecType = "root"
	ExecBlockType ExecType = "block"
)

type ResultInfo struct {
	Type        ExecType
	ExecName    string
	VertexName  string
	StartTime   time.Time
	EndTime     time.Time
	Input       input.Input
	Output      output.Output
	Success     bool
	Reason      string
	BlockResult Result
}

func New() Result {
	return &result{
		r: slice.New(),
	}
}

type result struct {
	r slice.Slice
}

func (r *result) Add(v any) {
	r.r.Add(v)
}

func (r *result) Get() []any {
	return r.r.Get()
}

func (r *result) Length() int {
	return r.r.Length()
}

func (r *result) Print() {
	totalSuccess := true
	var totalDuration time.Duration
	for i, v := range r.r.Get() {
		ri, ok := v.(*ResultInfo)
		if !ok {
			fmt.Printf("unexpected resultInfo, got %T\n", v)
		}
		if ri.Type == ExecRootType && ri.VertexName == "total" {
			totalDuration = ri.EndTime.Sub(ri.StartTime)
		} else {
			s := "OK"
			if !ri.Success {
				totalSuccess = false
				s = "NOK"
			}
			fmt.Printf("  result order: %d exec: %s vertex: %s, duration %s, success: %s, reason: %s\n",
				i,
				ri.ExecName,
				ri.VertexName,
				ri.EndTime.Sub(ri.StartTime),
				s,
				ri.Reason,
			)

			if ri.BlockResult != nil {
				ri.BlockResult.Print()
			}
		}
	}
	s := "OK"
	if !totalSuccess {
		s = "NOK"
	}
	fmt.Printf("overall result duration: %s, success: %s\n", totalDuration, s)
}
