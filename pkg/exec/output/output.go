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

package output

import (
	"encoding/json"
	"fmt"

	"github.com/fnrunner/fnutils/pkg/kv"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Output interface {
	kv.KV

	GetData(k string) any
	Print()
	GetFinalOutput() []any
	GetConditionedOutput() map[string]any
}

type OutputInfo struct {
	Internal    bool
	Conditioned bool
	GVK         *schema.GroupVersionKind
	Data        any
}

func New() Output {
	return &output{
		o: kv.New(),
	}
}

type output struct {
	o kv.KV
}

func (r *output) AddEntry(k string, v any) {
	//fmt.Printf("output: %s, value: %v\n", k, v)
	r.o.AddEntry(k, v)
}

func (r *output) DeleteEntry(k string) {
	r.o.DeleteEntry(k)
}

func (r *output) Add(o kv.KV) {
	if o == nil {
		return
	}
	r.o.Add(o)
}

func (r *output) Get() map[string]any {
	return r.o.Get()
}

func (r *output) GetValue(k string) any {
	return r.o.GetValue(k)
}

func (r *output) Length() int {
	return r.o.Length()
}

func (r *output) GetData(k string) any {
	v := r.o.GetValue(k)
	oi, ok := v.(*OutputInfo)
	if !ok {
		return nil
	}
	return oi.Data
}

// used for debugging purposes
func (r *output) Print() {
	for varName, v := range r.o.Get() {
		oi, ok := v.(*OutputInfo)
		if !ok {
			fmt.Printf("unexpected outputInfo, got %T\n", v)
			continue
		}
		b, err := json.Marshal(oi.Data)
		if err != nil {
			fmt.Printf("output %s: marshal err %s\n", varName, err.Error())
		}
		fmt.Printf("  json output varName: %s internal: %t gvk: %v value:%s\n", varName, oi.Internal, oi.GVK, string(b))
	}
}

func (r *output) GetFinalOutput() []any {
	fo := []any{}
	for _, v := range r.o.Get() {
		oi, ok := v.(*OutputInfo)
		if !ok {
			fmt.Printf("unexpected outputInfo, got %T\n", v)
			continue
		}
		if !oi.Internal {
			switch d := oi.Data.(type) {
			case []any:
				fo = append(fo, d...)
			}
		}
	}
	return fo
}

func (r *output) GetConditionedOutput() map[string]any {
	co := map[string]any{}
	for k, v := range r.o.Get() {
		oi, ok := v.(*OutputInfo)
		if !ok {
			fmt.Printf("unexpected outputInfo, got %T\n", v)
			continue
		}
		if oi.Conditioned {
			co[k] = v
		}
	}
	return co
}
