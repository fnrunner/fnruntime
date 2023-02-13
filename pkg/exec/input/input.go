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

package input

import (
	"encoding/json"
	"fmt"

	"github.com/fnrunner/fnutils/pkg/kv"
)

type Input interface {
	kv.KV

	//Print(s string)
}

func New() Input {
	return &input{
		i: kv.New(),
	}
}

type input struct {
	i kv.KV
}

func (r *input) AddEntry(k string, v any) {
	r.i.AddEntry(k, v)
}

func (r *input) DeleteEntry(k string) {
	r.i.DeleteEntry(k)
}

func (r *input) Add(i kv.KV) {
	r.i.Add(i)
}

func (r *input) Get() map[string]any {
	return r.i.Get()
}

func (r *input) GetValue(k string) any {
	return r.i.GetValue(k)
}

func (r *input) Length() int {
	return r.i.Length()
}

func (r *input) Print(vertexName string) {
	for varName, v := range r.i.Get() {
		b, err := json.Marshal(v)
		if err != nil {
			fmt.Printf("input vertexName: %s, varName: %s: marshal err %s\n", vertexName, varName, err.Error())
		}
		fmt.Printf("json input vertexName: %s, varName: %s value:%s\n", vertexName, varName, string(b))
	}
}
