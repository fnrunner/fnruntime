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

package fnctrlrreconciler

import (
	"context"
	"fmt"
	"time"

	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	"github.com/fnrunner/fnruntime/pkg/ctrlr/controllers/reconciler"
	"github.com/fnrunner/fnruntime/pkg/ctrlr/fncontroller"
	"github.com/fnrunner/fnruntime/pkg/fnmanager/fnreconciler"
	"github.com/fnrunner/fnruntime/pkg/imgmanager/imgmanager"
	"github.com/fnrunner/fnruntime/pkg/store/ctrlstore"
	ctrlcfgv1alpha1 "github.com/fnrunner/fnsyntax/apis/controllerconfig/v1alpha1"
	"github.com/fnrunner/fnsyntax/pkg/ccsyntax"
	"github.com/fnrunner/fnutils/pkg/meta"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	defaultConfigMapKey = "controllerConfig"
	cmLabelKey          = "fnrun.io/configmap"
	finalizer           = "fnrun.io/finalizer"
	defaultTimeout      = time.Second * 5
)

type Config struct {
	Client          *kubernetes.Clientset
	Mgr             manager.Manager
	ControllerStore ctrlstore.Store
}

func New(cfg *Config) fnreconciler.Reconciler {
	l := ctrl.Log.WithName("fn reconciler")
	return &rec{
		client:    cfg.Client,
		ctrlStore: cfg.ControllerStore,
		mgr:       cfg.Mgr,
		ge:        make(chan event.GenericEvent),
		l:         l,
	}
}

type rec struct {
	client    *kubernetes.Clientset
	ctrlStore ctrlstore.Store
	mgr       manager.Manager
	fnc       fncontroller.Controller
	fni       imgmanager.Manager
	key       string
	ge        chan event.GenericEvent
	cm        *corev1.ConfigMap // keeps track of the last known good configmap which whom we operate
	l         logr.Logger
}

func (r *rec) Reconcile(ctx context.Context, key types.NamespacedName) (bool, error) {

	r.l.WithValues("key", key)

	cm, err := r.client.CoreV1().ConfigMaps(key.Namespace).Get(ctx, key.Name, metav1.GetOptions{})
	if err != nil {
		if meta.IgnoreNotFound(err) != nil {
			return false, err
		}
		return true, nil
	}
	meta.AddLabels(cm, map[string]string{cmLabelKey: r.getLabelValue(key)})
	meta.AddFinalizer(cm, finalizer)

	// configMap was deleted
	if meta.WasDeleted(cm) {
		// delete/stop the controller
		r.l.Info("configmap delete -> stop controller...")
		if r.fni != nil {
			r.fni.Stop()
		}
		if r.fnc != nil {
			r.fnc.Stop(ctx)
		}
		meta.RemoveFinalizer(cm, finalizer)
		if _, err := r.client.CoreV1().ConfigMaps(key.Namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
			r.l.Error(err, "cannot update resource")
			return false, err
		}
		return false, nil
	}

	// action is either ignote
	action := r.checkAction(cm)
	if action == Ignore {
		// if the fncontroller is running all ok, if not we need to run it
		if r.fnc != nil && r.fnc.IsRunning() {
			r.l.Info("configmap update -> ignore controller update...")
			if _, err := r.client.CoreV1().ConfigMaps(key.Namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
				r.l.Error(err, "cannot update resource")
				return false, err
			}
			return false, nil
		}
	}
	// get the ceCtx
	images, ceCtx, err := r.getExecCtxAndImages(cm)
	if err != nil {
		r.l.Error(err, "cannot run controller with this execution context")
		// new execution context is nok
		// do we stop the controllers or do we keep running ??? TBD
		return false, err
	}
	if action == Update {
		// delete/stop the controller
		r.l.Info("configmap update -> stop controller...")
		if r.fni != nil {
			r.fni.Stop()
		}
		if r.fnc != nil {
			r.fnc.Stop(ctx)
		}
	}
	// create the fn image manager
	r.fni, err = imgmanager.New(&imgmanager.Config{
		ControllerStore: r.ctrlStore,
		Client:          r.client,
		ControllerName:  key.Name,
		Namespace:       key.Namespace,
		Images:          images,
		ConfigMap:       cm, // use the latest cm
	})
	if err != nil {
		r.l.Error(err, "cannot create img manager")
		return false, err
	}
	if err := r.fni.Start(ctx); err != nil {
		r.l.Error(err, "cannot start fn proxy")
		return false, err
	}

	// create the controller
	r.fnc = fncontroller.New(r.mgr, ceCtx, r.ge)
	// start the controller
	r.l.Info("start controller...")
	if err := r.fnc.Start(ctx, cm.Name, controller.Options{
		Reconciler: reconciler.New(&reconciler.Config{
			Client:       r.mgr.GetClient(),
			PollInterval: 1 * time.Minute,
			CeCtx:        ceCtx,
		}),
	}); err != nil {
		r.l.Error(err, "cannot start controller")
		return false, err
	}

	if _, err := r.client.CoreV1().ConfigMaps(key.Namespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		r.l.Error(err, "cannot update resource")
		return false, err
	}
	r.cm = cm
	return false, nil
}

func (r *rec) getLabelValue(key types.NamespacedName) string {
	return fmt.Sprintf("%s-%s", key.Namespace, key.Name)
}

func (r *rec) getExecCtxAndImages(cm *corev1.ConfigMap) ([]*fnrunv1alpha1.Image, ccsyntax.ConfigExecutionContext, error) {
	ctrlcfg := &ctrlcfgv1alpha1.ControllerConfigSpec{}
	if err := yaml.Unmarshal([]byte(cm.Data[r.key]), ctrlcfg); err != nil {
		r.l.Error(err, "cannot unmarshal")
		return nil, nil, err
	}

	p, result := ccsyntax.NewParser(ctrlcfg)
	if len(result) > 0 {
		err := fmt.Errorf("failed ccsyntax validation, result %v", result)
		r.l.Error(err, "syntax validation faile")
		return nil, nil, err
	}
	r.l.Info("ccsyntax validation succeeded")

	ceCtx, result := p.Parse()
	if len(result) != 0 {
		err := fmt.Errorf("failed ccsyntax parsing, result %v", result)
		for _, res := range result {
			r.l.Error(err, "ccsyntax parsing failed", "result", res)
		}
		return nil, nil, err
	}
	r.l.Info("ccsyntax parsing succeeded")
	return p.GetImages(), ceCtx, nil
}

type Action int

const (
	Create Action = iota
	Update
	Ignore
)

func (r *rec) checkAction(cm *corev1.ConfigMap) Action {
	if r.cm == nil {
		// create
		return Create
	}
	if r.cm.Data[r.key] != cm.Data[r.key] {
		return Update
	}
	return Ignore
}
