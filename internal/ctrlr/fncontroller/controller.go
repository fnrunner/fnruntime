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

package fncontroller

import (
	"context"
	"fmt"

	"github.com/fnrunner/fnruntime/internal/ctrlr/controllers/eventhandler"
	"github.com/fnrunner/fnsyntax/pkg/ccsyntax"
	"github.com/fnrunner/fnutils/pkg/meta"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	errCreateCache      = "cannot create cache"
	errStartCache       = "cannot start cache/crash cache"
	errCreateController = "cannot create controller"
	errStartController  = "cannot start controller/crash controller"
	errCreateWatch      = "cannot create watch"
)

type FnController interface {
	IsRunning() bool
	Error() error
	Start(ctx context.Context, name string, o controller.Options) error
	Stop()
}

type fnctrlr struct {
	mgr   manager.Manager
	ceCtx ccsyntax.ConfigExecutionContext
	ge    chan event.GenericEvent

	globalPredicates []predicate.Predicate

	cancel context.CancelFunc
	err    error
	l      logr.Logger
}

func New(mgr manager.Manager, ceCtx ccsyntax.ConfigExecutionContext, ge chan event.GenericEvent) FnController {
	return &fnctrlr{
		mgr:   mgr,
		ceCtx: ceCtx,
		ge:    ge,
		// initialize
		globalPredicates: []predicate.Predicate{},
		cancel:           nil,
		err:              nil,
	}
}

func (r *fnctrlr) IsRunning() bool {
	return r.cancel != nil
}

func (r *fnctrlr) Error() error {
	return r.err
}

func (r *fnctrlr) Stop() {
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
}

func (r *fnctrlr) Start(ctx context.Context, name string, o controller.Options) error {
	r.l = log.FromContext(ctx).WithValues("name", name)
	r.l.Info("start cfgCtrl")
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	cache, err := cache.New(r.mgr.GetConfig(), cache.Options{Scheme: r.mgr.GetScheme(), Mapper: r.mgr.GetRESTMapper()})
	if err != nil {
		r.l.Error(err, errCreateCache)
		return fmt.Errorf("%s err: %s", errCreateCache, err)
	}

	ctrl, err := controller.NewUnmanaged(name, r.mgr, o)
	if err != nil {
		r.l.Error(err, errCreateController)
		return fmt.Errorf("%s, err: %s", errCreateController, err)
	}

	// For watch
	hdler := &handler.EnqueueRequestForObject{}
	forSrcKind := meta.GetUnstructuredFromGVK(r.ceCtx.GetForGVK())
	allPredicates := append(r.globalPredicates, []predicate.Predicate{}...)
	if err := ctrl.Watch(
		source.NewKindWithCache(forSrcKind, cache),
		hdler,
		allPredicates...); err != nil {
		return fmt.Errorf("%s, err: %s", errCreateWatch, err)
	}

	// Generic Event watch
	if err := ctrl.Watch(
		&source.Channel{Source: r.ge},
		hdler,
		allPredicates...); err != nil {
		return fmt.Errorf("%s, err: %s", errCreateWatch, err)
	}

	// own watch
	for gvk := range r.ceCtx.GetFOW(ccsyntax.FOWOwn) {
		src := &source.Kind{Type: meta.GetUnstructuredFromGVK(&gvk)}
		hdlr := &handler.EnqueueRequestForOwner{
			OwnerType:    forSrcKind,
			IsController: true,
		}
		allPredicates := append([]predicate.Predicate(nil), r.globalPredicates...)
		allPredicates = append(allPredicates, []predicate.Predicate{}...)
		if err := ctrl.Watch(
			src,
			hdlr,
			allPredicates...,
		); err != nil {
			return fmt.Errorf("%s, err: %s", errCreateWatch, err)
		}
	}
	// watch watch
	for gvk, od := range r.ceCtx.GetFOW(ccsyntax.FOWWatch) {
		//var obj client.Object
		obj := meta.GetUnstructuredFromGVK(&gvk)

		allPredicates := append([]predicate.Predicate(nil), r.globalPredicates...)
		allPredicates = append(allPredicates, []predicate.Predicate{}...)

		// If the source of this watch is of type *source.Kind, project it.
		src := &source.Kind{Type: obj}

		eh := eventhandler.New(&eventhandler.Config{
			Client:         r.mgr.GetClient(),
			RootVertexName: od[ccsyntax.OperationApply].RootVertexName,
			GVK:            &gvk,
			DAG:            od[ccsyntax.OperationApply].DAG,
		})

		if err := ctrl.Watch(src, eh, allPredicates...); err != nil {
			return err
		}
	}

	go func() {
		<-r.mgr.Elected()
		r.err = cache.Start(ctx)
		if r.err != nil {
			r.l.Error(err, errStartCache)
		}

		if r.cancel != nil {
			r.cancel()
		}
	}()
	go func() {
		<-r.mgr.Elected()
		r.err = ctrl.Start(ctx)
		if r.err != nil {
			r.l.Error(err, errStartController)
		}
		if r.cancel != nil {
			r.cancel()
		}
	}()
	return nil
}
