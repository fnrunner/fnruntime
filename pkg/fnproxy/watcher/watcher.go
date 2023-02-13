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

package watcher

/*
import (
	"context"
	"time"

	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	defaultWaitTime = time.Second * 5
)

type Watcher interface {
	Start(ctx context.Context)
	Stop()
	Query(ctx context.Context) (string, error)
}

type CreateClientFn func(image fnrunv1alpha1.Image, podIP string)
type DeleteCLientFn func(image fnrunv1alpha1.Image)

type Config struct {
	ClientSet      *kubernetes.Clientset
	Image          fnrunv1alpha1.Image
	Namespace      string
	PodName        string
	CreateClientFn CreateClientFn
	DeleteClientFn DeleteCLientFn
}

func New(cfg *Config) Watcher {
	l := ctrl.Log.WithName("watcher")
	return &watcher{
		clientset:      cfg.ClientSet,
		image:          cfg.Image,
		namespace:      cfg.Namespace,
		podName:        cfg.PodName,
		createClientFn: cfg.CreateClientFn,
		deleteClientFn: cfg.DeleteClientFn,
		l:              l,
	}

}

type watcher struct {
	clientset      *kubernetes.Clientset
	image          fnrunv1alpha1.Image
	namespace      string
	podName        string
	cancel         context.CancelFunc
	createClientFn CreateClientFn
	deleteClientFn DeleteCLientFn
	l              logr.Logger
}

func (r *watcher) Start(ctx context.Context) {
	r.l.WithValues("image", r.image)
	r.l.Info("start watcher")
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.l.Info("start watching", "podName", r.podName)
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			fnrunv1alpha1.FunctionLabelKey: r.podName,
		}}
WATCH:
	wi, err := r.clientset.CoreV1().Pods(r.namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Watch:         true,
	})
	if err != nil {
		r.l.Error(err, "cannot create watch")
		time.Sleep(defaultWaitTime)
		goto WATCH
	}
	for {
		select {
		case _, ok := <-wi.ResultChan():
			if !ok {
				goto WATCH
			}
			// updates the client if the
			podIP, err := r.Query(ctx)
			if err != nil {
				r.l.Error(err, "cannot query podIP")
				// TODO add recreate POD + SVC
				// callback
				r.deleteClientFn(r.image)
				goto WATCH
			}
			if podIP == "" {
				// callback
				r.deleteClientFn(r.image)
			} else {
				//  callback
				r.createClientFn(r.image, podIP)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (r *watcher) Stop() {
	r.l.WithValues("image", r.image)
	r.l.Info("stop watcher")
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *watcher) Query(ctx context.Context) (string, error) {
	r.l.WithValues("image", r.image)
	r.l.Info("query")
	r.l.Info("start watching", "podName", r.podName)
	pod, err := r.clientset.CoreV1().Pods(r.namespace).Get(ctx, r.podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if pod.Status.Phase != "Running" {
		return "", err
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return pod.Status.PodIP, nil
		}
	}
	return "", nil
}
*/
