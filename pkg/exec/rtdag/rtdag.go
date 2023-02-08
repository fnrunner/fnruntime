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

package rtdag

import (
	"fmt"
	"os"
	"sync"

	"github.com/fnrunner/fnruntime/pkg/exec/output"
	ctrlcfgv1alpha1 "github.com/fnrunner/fnsyntax/apis/controllerconfig/v1alpha1"
	"github.com/fnrunner/fnutils/pkg/dag"
)

type RuntimeDAG interface {
	dag.DAG

	GetRootVertex() string
	GetDependencyMap(from string)
	PrintVertices()
}

func New() RuntimeDAG {
	return &runtimeDAG{
		d: dag.New(),
	}
}

type runtimeDAG struct {
	d dag.DAG
}

type VertexKind string

const (
	RootVertexKind     VertexKind = "root"
	FunctionVertexKind VertexKind = "function"
)

type VertexContext struct {
	m          sync.Mutex
	VertexName string     // vertexName
	Kind       VertexKind // kind of the vertex

	// the below elements provide information that is needed during the runtime operation
	BlockDAG   RuntimeDAG
	Function   ctrlcfgv1alpha1.Function
	References []string
	// contains the information about the output to ease the reverse mapping
	// of the output the function provides to the output we produce
	Outputs      output.Output
	GVKToVarName map[string]string
}

func (r *runtimeDAG) AddVertex(s string, v any) error {
	return r.d.AddVertex(s, v)
}

func (r *runtimeDAG) Connect(from, to string) {
	r.d.Connect(from, to)
}

func (r *runtimeDAG) AddDownEdge(from, to string) {
	r.d.AddDownEdge(from, to)
}

func (r *runtimeDAG) AddUpEdge(from, to string) {
	r.d.AddUpEdge(from, to)
}

func (r *runtimeDAG) VertexExists(s string) bool {
	return r.d.VertexExists(s)
}

func (r *runtimeDAG) GetVertex(s string) any {
	return r.d.GetVertex(s)
}

func (r *runtimeDAG) GetVertices() map[string]any {
	return r.d.GetVertices()
}

func (r *runtimeDAG) GetDownVertexes(from string) []string {
	return r.d.GetDownVertexes(from)
}

func (r *runtimeDAG) GetUpVertexes(from string) []string {
	return r.d.GetUpVertexes(from)

}

func (r *runtimeDAG) TransitiveReduction() {
	r.d.TransitiveReduction()
}

func (r *VertexContext) AddReference(s string) {
	r.m.Lock()
	defer r.m.Unlock()
	found := false
	for _, ref := range r.References {
		if ref == s {
			found = true
		}
	}
	if !found {
		r.References = append(r.References, s)
	}
}

func (r *runtimeDAG) GetRootVertex() string {
	for vertexName, v := range r.d.GetVertices() {
		vc, ok := v.(*VertexContext)
		if ok {
			if vc.Kind == RootVertexKind {
				return vertexName
			}
		}
	}
	return ""
}

func (r *runtimeDAG) GetDependencyMap(from string) {
	fmt.Println("######### dependency map verteces start ###########")
	for vertexName := range r.GetVertices() {
		fmt.Printf("%s\n", vertexName)
	}
	fmt.Println("######### dependency map verteces end ###########")
	fmt.Println("######### dependency map start ###########")
	r.getDependencyMap(from, 0)
	fmt.Println("######### dependency map end   ###########")
}

func (r *runtimeDAG) getDependencyMap(from string, indent int) {
	fmt.Printf("%s:\n", from)
	for _, upVertex := range r.GetUpVertexes(from) {
		found := r.checkVertex(upVertex)
		if !found {
			fmt.Printf("upVertex %s no found in vertices\n", upVertex)
			os.Exit(1)
		}
		fmt.Printf("-> %s\n", upVertex)
	}
	indent++
	for _, downVertex := range r.GetDownVertexes(from) {
		found := r.checkVertex(downVertex)
		if !found {
			fmt.Printf("upVertex %s no found in vertices\n", downVertex)
			os.Exit(1)
		}
		r.getDependencyMap(downVertex, indent)
	}
}

func (r *runtimeDAG) checkVertex(s string) bool {
	for vertexName := range r.GetVertices() {
		if vertexName == s {
			return true
		}
	}
	return false
}

func (r *runtimeDAG) PrintVertices() {
	fmt.Printf("###### RUNTIME DAG output start #######\n")
	for vertexName, v := range r.GetVertices() {
		vc, ok := v.(*VertexContext)
		if ok {
			fmt.Printf("vertexname: %s upVertices: %v, downVertices: %v\n", vertexName, r.GetUpVertexes(vertexName), r.GetDownVertexes(vertexName))
			vc.Outputs.Print()
		} else {
			fmt.Printf("vertexname: %s wrong context\n", vertexName)
		}
	}
	fmt.Printf("###### RUNTIME DAG output stop #######\n")
}
