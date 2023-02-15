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

package fnctrlrcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/fnrunner/fnruntime/pkg/fnmanager/fnreconciler"
	"github.com/fnrunner/fnruntime/pkg/store/ctrlstore"
	"github.com/fnrunner/fnutils/pkg/meta"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	defaultWaitTime = time.Second * 5
	finalizer       = "fnrun.io/finalizer"
	cmLabelKey      = "fnrun.io/configmap"
)

type Controller interface {
	Start(ctx context.Context) error
}

type Config struct {
	Name            string // controllername
	Namespace       string
	ControllerStore ctrlstore.Store
	Client          *kubernetes.Clientset
	Reconciler      fnreconciler.Reconciler
}

func New(cfg *Config) Controller {
	l := ctrl.Log.WithName("fn ctrlr controller")

	reconciler := fnreconciler.NewNopReconciler()
	if cfg.Reconciler != nil {
		reconciler = cfg.Reconciler
	}

	return &fnctrlctrlr{
		name:       cfg.Name,
		namespace:  cfg.Namespace,
		errChan:    make(chan error),
		ctrlStore:  cfg.ControllerStore,
		client:     cfg.Client,
		reconciler: reconciler,
		l:          l,
	}
}

type fnctrlctrlr struct {
	name       string // controllername
	namespace  string
	client     *kubernetes.Clientset
	errChan    chan error
	ctrlStore  ctrlstore.Store
	reconciler fnreconciler.Reconciler

	l logr.Logger
}

func (r *fnctrlctrlr) Start(ctx context.Context) error {
	for {
		select {
		default:
		INIT:
			cm, err := r.get(ctx)
			if err != nil {
				//r.l.Error(err, "cannot get cm")
				time.Sleep(defaultWaitTime)
				goto INIT
			}
			// we add a label so the watch can use a label selector
			cm, err = r.update(ctx, cm)
			if err != nil {
				//r.l.Error(err, "cannot update cm")
				time.Sleep(defaultWaitTime)
				goto INIT
			}
			if err := r.start(ctx); err != nil {
				//r.l.Error(err, "watch failed")
				time.Sleep(defaultWaitTime)
				goto INIT
			}
			goto INIT
		case <-ctx.Done():
			// We are done
			return nil
		}
	}
}

func (r *fnctrlctrlr) start(ctx context.Context) error {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			cmLabelKey: r.getLabelValue(),
		}}
	wi, err := r.client.CoreV1().ConfigMaps(r.namespace).Watch(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
			Watch:         true,
		})
	if err != nil {
		r.l.Error(err, "cannot create watch")
		return err
	}
	for {
		select {
		case we, ok := <-wi.ResultChan():
			// Channel is closed.
			if !ok {
				err := fmt.Errorf("watch channel closed, we: %v", we)
				r.l.Error(err, "watch channel closed")
				return err
			}
			done, err := r.reconciler.Reconcile(ctx, types.NamespacedName{Namespace: r.namespace, Name: r.name})
			if err != nil {
				r.l.Error(err, "cannot reconcile")
				// TBD what to do here, we should requeue
			}
			if done {
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (r *fnctrlctrlr) get(ctx context.Context) (*corev1.ConfigMap, error) {
	// get configmap
	cm, err := r.client.CoreV1().ConfigMaps(r.namespace).Get(ctx, r.name, metav1.GetOptions{})
	if err != nil {
		if meta.IgnoreNotFound(err) == nil {
			r.l.Info("configmap not found")
		} else {
			r.l.Error(err, "cannot get configmap")
		}
		return nil, err
	}
	//r.l.Info("configmap", "data", cm.Data)
	return cm, nil
}

func (r *fnctrlctrlr) update(ctx context.Context, cm *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	// update configmap to ensure the label is set properly and the watch can operate on this
	meta.AddLabels(cm, map[string]string{cmLabelKey: r.getLabelValue()})

	// add finalizer so that a delete can cleanup the resources
	meta.AddFinalizer(cm, finalizer)

	// update the cm
	return r.client.CoreV1().ConfigMaps(r.namespace).Update(ctx, cm, metav1.UpdateOptions{})
}

func (r *fnctrlctrlr) getLabelValue() string {
	return fmt.Sprintf("%s-%s", r.namespace, r.name)
}
