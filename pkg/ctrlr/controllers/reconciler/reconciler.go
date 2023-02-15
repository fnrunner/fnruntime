/*
Copyright 2022 Nokia.

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

package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fnrunner/fnproto/pkg/executor/execclient"
	"github.com/fnrunner/fnproto/pkg/service/svcclient"
	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/fnrunner/fnruntime/internal/ctrlr/event"
	"github.com/fnrunner/fnruntime/pkg/fnproxy/clients"
	"github.com/fnrunner/fnruntime/pkg/exec/builder"
	"github.com/fnrunner/fnruntime/pkg/exec/fnmap"
	"github.com/fnrunner/fnruntime/pkg/exec/output"
	"github.com/fnrunner/fnruntime/pkg/exec/result"
	"github.com/fnrunner/fnsyntax/pkg/ccsyntax"
	"github.com/fnrunner/fnutils/pkg/applicator"
	"github.com/fnrunner/fnutils/pkg/meta"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

const (
	// const
	defaultFinalizerName = "fnrun.io/finalizer"
	// errors
	errGetCr        = "cannot get resource"
	errUpdateStatus = "cannot update resource status"
	errMarshalCr    = "cannot marshal resource"

// reconcileFailed = "reconcile failed"

)

type Config struct {
	Client       client.Client
	PollInterval time.Duration
	CeCtx        ccsyntax.ConfigExecutionContext
	FnMap        fnmap.FuncMap
}

func New(c *Config) reconcile.Reconciler {
	/*
		opts := zap.Options{
			Development: true,
		}
		ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	*/

	return &reconciler{
		client:       applicator.ClientApplicator{Client: c.Client, Applicator: applicator.NewAPIPatchingApplicator(c.Client)},
		pollInterval: c.PollInterval,
		ceCtx:        c.CeCtx,
		fnMap:        c.FnMap,
		l:            ctrl.Log.WithName("fnrun reconcile"),
		f:            meta.NewAPIFinalizer(c.Client, defaultFinalizerName),
		record:       event.NewNopRecorder(),
	}
}

type reconciler struct {
	client       applicator.ClientApplicator
	pollInterval time.Duration
	ceCtx        ccsyntax.ConfigExecutionContext
	fnMap        fnmap.FuncMap
	f            meta.Finalizer
	l            logr.Logger
	record       event.Recorder
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.l = log.FromContext(ctx)
	r.l.Info("reconcile start...")

	gvk := r.ceCtx.GetForGVK()
	//o := getUnstructured(r.gvk)
	cr := meta.GetUnstructuredFromGVK(gvk)
	if err := r.client.Get(ctx, req.NamespacedName, cr); err != nil {
		// if the CR no longer exist we are done
		r.l.Info(errGetCr, "error", err)
		return reconcile.Result{}, errors.Wrap(meta.IgnoreNotFound(err), errGetCr)
	}

	//record := r.record.WithAnnotations()

	x, err := meta.MarshalData(cr)
	if err != nil {
		r.l.Error(err, "cannot marshal data")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.f.AddFinalizer(ctx, cr); err != nil {
		r.l.Error(err, "cannot add finalizer")
		//managed.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
		return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}

	fnc, err := r.getFnClients()
	if err != nil {
		r.l.Error(err, "get svc clients")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
	}
	defer fnc.Execclient.Close()
	defer fnc.Svcclient.Close()

	// delete branch -> used for delete
	if meta.WasDeleted(cr) {
		r.l.Info("reconcile delete started...")
		// handle delete branch
		deleteDAGCtx := r.ceCtx.GetDAGCtx(ccsyntax.FOWFor, gvk, ccsyntax.OperationDelete)

		o := output.New()
		result := result.New()
		e := builder.New(&builder.Config{
			Name:      req.Name,
			Namespace: req.Namespace,
			ControllerName: r.ceCtx.GetName(),
			Data:      x,
			Client:    r.client,
			GVK:       gvk,
			DAG:       deleteDAGCtx.DAG,
			Output:    o,
			Result:    result,
			FnClients: fnc,
		})

		// TODO should be per crName
		e.Run(ctx)
		//o.Print()
		result.Print()

		if err := r.f.RemoveFinalizer(ctx, cr); err != nil {
			r.l.Error(err, "cannot remove finalizer")
			//managed.SetConditions(nddv1.ReconcileError(err), nddv1.Unknown())
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}

		r.l.Info("reconcile delete finished...")

		return reconcile.Result{}, nil
	}
	// apply branch -> used for create and update
	r.l.Info("reconcile apply started...")
	applyDAGCtx := r.ceCtx.GetDAGCtx(ccsyntax.FOWFor, gvk, ccsyntax.OperationApply)

	o := output.New()
	result := result.New()
	e := builder.New(&builder.Config{
		Name:      req.Name,
		Namespace: req.Namespace,
		ControllerName: r.ceCtx.GetName(),
		Data:      x,
		Client:    r.client,
		GVK:       gvk,
		DAG:       applyDAGCtx.DAG,
		Output:    o,
		Result:    result,
		FnClients: fnc,
	})

	e.Run(ctx)
	//o.Print()
	result.Print()

	// TODO check result if failed, return an error

	for _, output := range o.GetFinalOutput() {
		b, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			r.l.Error(err, "cannot marshal the content")
			return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}
		//r.l.Info("final output", "jsin string", string(b))
		u := &unstructured.Unstructured{}
		if err := json.Unmarshal(b, u); err != nil {
			r.l.Error(err, "cannot unmarshal the content")
			return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
		}
		r.l.Info("final output", "unstructured", u)

		r.l.Info("gvk", "cr", cr.GroupVersionKind(), "u", u.GroupVersionKind())

		if u.GroupVersionKind() == cr.GroupVersionKind() {
			cr = u
		} else {
			if err := r.client.Apply(ctx, u); err != nil {
				r.l.Error(err, "cannot apply the content")
				return reconcile.Result{RequeueAfter: 5 * time.Second}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
			}
		}
	}

	r.l.Info("reconcile apply finished...")
	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) getFnClients() (*clients.Clients, error) {
	svcClient, err := svcclient.New(&svcclient.Config{
		Address:  fmt.Sprintf("%s:%d", "127.0.0.1", fnrunv1alpha1.FnProxyGRPCServerPort),
		Insecure: true,
	})
	if err != nil {
		r.l.Error(err, "cannot create new client")
		return nil, err
	}

	execClient, err := execclient.New(&execclient.Config{
		Address:  fmt.Sprintf("%s:%d", "127.0.0.1", fnrunv1alpha1.FnProxyGRPCServerPort),
		Insecure: true,
	})
	if err != nil {
		r.l.Error(err, "cannot create new client")
		return nil, err
	}
	return &clients.Clients{
		Execclient: execClient,
		Svcclient:  svcClient,
	}, nil
}
