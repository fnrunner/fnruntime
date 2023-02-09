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

package eventhandler

import (
	"context"

	"github.com/fnrunner/fnruntime/pkg/exec/builder"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnruntime/pkg/exec/rtdag"
	"github.com/fnrunner/fnutils/pkg/meta"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type Config struct {
	Client         client.Client
	RootVertexName string
	GVK            *schema.GroupVersionKind
	DAG            rtdag.RuntimeDAG
}

func New(c *Config) handler.EventHandler {
	/*
		opts := zap.Options{
			Development: true,
		}
		ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	*/

	return &eventhandler{
		//ctx:    ctx,
		client:         c.Client,
		rootVertexName: c.RootVertexName,
		gvk:            c.GVK,
		d:              c.DAG,
		l:              ctrl.Log.WithName("fnrun eventhandler"),
	}
}

type eventhandler struct {
	client client.Client
	//ctx    context.Context
	rootVertexName string
	gvk            *schema.GroupVersionKind
	d              rtdag.RuntimeDAG

	l logr.Logger
}

// Create enqueues a request for all infrastructures which pertains to the topology.
func (r *eventhandler) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	r.add(evt.Object, q)
}

// Create enqueues a request for all infrastructures which pertains to the topology.
func (r *eventhandler) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	r.add(evt.ObjectOld, q)
	r.add(evt.ObjectNew, q)
}

// Create enqueues a request for all infrastructures which pertains to the topology.
func (r *eventhandler) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	r.add(evt.Object, q)
}

// Create enqueues a request for all infrastructures which pertains to the topology.
func (r *eventhandler) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	r.add(evt.Object, q)
}

func (r *eventhandler) add(obj runtime.Object, queue adder) {
	r.l.Info("watch event started...")

	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	x, err := meta.MarshalData(u)
	if err != nil {
		r.l.Error(err, "cannot marshal data")
		return
	}

	namespace := u.GetNamespace()
	if u.GetNamespace() == "" {
		namespace = "default"
	}

	o := output.New()
	result := result.New()
	e := builder.New(&builder.Config{
		Name:      u.GetName(),
		Namespace: namespace,
		Data:      x,
		Client:    r.client,
		GVK:       r.gvk,
		DAG:       r.d,
		Output:    o,
		Result:    result,
	})

	e.Run(context.TODO())
	//o.Print()
	result.Print()

	// for all the output add the queues

	/*
		queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: toponode.GetNamespace(),
			Name:      toponode.GetName()}},
		)
	*/

	r.l.Info("watch event finsihed...")
}

type adder interface {
	Add(item interface{})
}
