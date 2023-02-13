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

package builder

import (
	"github.com/fnrunner/fnruntime/pkg/fnproxy/clients"
	"github.com/fnrunner/fnruntime/pkg/exec/exechandler"
	"github.com/fnrunner/fnruntime/pkg/exec/fnmap"
	"github.com/fnrunner/fnruntime/pkg/exec/fnmap/functions"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	"github.com/fnrunner/fnutils/pkg/executor"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Config struct {
	Name      string
	Namespace string
	Data      any
	Client    client.Client
	GVK       *schema.GroupVersionKind
	DAG       rtdag.RuntimeDAG
	Output    output.Output
	Result    result.Result
	FnClients *clients.Clients
}

func New(c *Config) executor.Executor {
	rootVertexName := c.DAG.GetRootVertex()

	// create a new fn map
	fnmap := functions.Init(&fnmap.Config{
		Name:           c.Name,
		Namespace:      c.Namespace,
		RootVertexName: rootVertexName,
		Client:         c.Client,
		Output:         c.Output,
		Result:         c.Result,
		FnClients:      c.FnClients,
	})

	// Initialize the initial data
	c.Output.AddEntry(rootVertexName, &output.OutputInfo{
		Internal: true,
		GVK:      c.GVK,
		Data:     c.Data,
	})

	// initialize the handler
	h := exechandler.New(&exechandler.Config{
		Name:   rootVertexName,
		Type:   result.ExecRootType,
		DAG:    c.DAG,
		FnMap:  fnmap,
		Output: c.Output,
		Result: c.Result,
	})

	return executor.New(c.DAG, &executor.Config{
		Name:               rootVertexName,
		From:               rootVertexName,
		VertexFuntionRunFn: h.FunctionRun,
		ExecPostRunFn:      h.RecordFinalResult,
	})
}
