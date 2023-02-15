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

package fnctrlrmanager

import (
	"context"

	"github.com/fnrunner/fnruntime/pkg/fnmanager/fnctrlrmanager/fnctrlrcontroller"
	"github.com/fnrunner/fnruntime/pkg/fnmanager/fnctrlrmanager/fnctrlrreconciler"
	"github.com/fnrunner/fnruntime/pkg/store/ctrlstore"
	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Manager interface {
	Start(ctx context.Context) error
}

type Config struct {
	ControllerStore ctrlstore.Store
	Client          *kubernetes.Clientset
	Namespace       string
	Manager         manager.Manager
}

func New(cfg *Config) Manager {
	l := ctrl.Log.WithName("fn ctrlr manager")

	return &fnctrlmgr{
		errChan:   make(chan error),
		ctrlStore: cfg.ControllerStore,
		client:    cfg.Client,
		namespace: cfg.Namespace,
		mgr:       cfg.Manager,
		l:         l,
	}
}

type fnctrlmgr struct {
	errChan   chan error
	ctrlStore ctrlstore.Store
	client    *kubernetes.Clientset
	namespace string
	mgr       manager.Manager
	l         logr.Logger
}

func (r *fnctrlmgr) Start(ctx context.Context) error {
	for _, controllerName := range r.ctrlStore.List() {
		// create a fnctrlrcontroller
		fncc := fnctrlrcontroller.New(&fnctrlrcontroller.Config{
			Name:            controllerName,
			Namespace:       r.namespace,
			ControllerStore: r.ctrlStore,
			Client:          r.client,
			Reconciler: fnctrlrreconciler.New(&fnctrlrreconciler.Config{
				Client: r.client,
				Mgr:    r.mgr,
			}),
		})

		// start a fnctrlcontroller
		go func() {
			if err := fncc.Start(ctx); err != nil {
				r.errChan <- err
			}
		}()

	}
	for {
		select {
		case <-ctx.Done():
			// We are done
			return nil
		case err := <-r.errChan:
			// Error starting or during start
			return err
		}
	}
}
