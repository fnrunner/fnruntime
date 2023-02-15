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

package imgmanager

import (
	"context"
	"fmt"

	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	"github.com/fnrunner/fnruntime/pkg/imgmanager/imgcontroller"
	"github.com/fnrunner/fnruntime/pkg/store/ctrlstore"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Manager interface {
	Start(ctx context.Context) error
	Stop()
}

type Config struct {
	ControllerStore ctrlstore.Store
	Client          *kubernetes.Clientset
	ControllerName  string
	Namespace       string
	Images          []*fnrunv1alpha1.Image
	ConfigMap       *corev1.ConfigMap
}

func New(cfg *Config) (Manager, error) {
	l := ctrl.Log.WithName("img manager").WithValues("controller", cfg.ControllerName)

	imageStore := cfg.ControllerStore.GetImageStore(cfg.ControllerName)
	if imageStore == nil {
		return nil, fmt.Errorf("cannot create img manager, respective controller not initialize in store")
	}
	for _, image := range cfg.Images {
		imageStore.Create(*image)
	}
	return &imgmgr{
		controllerName: cfg.ControllerName,
		errChan:        make(chan error),
		ctrlStore:      cfg.ControllerStore,
		client:         cfg.Client,
		namespace:      cfg.Namespace,
		cm:             cfg.ConfigMap,
		l:              l,
	}, nil
}

type imgmgr struct {
	controllerName string
	errChan        chan error
	ctrlStore      ctrlstore.Store
	client         *kubernetes.Clientset
	namespace      string
	//mgr            manager.Manager
	cm     *corev1.ConfigMap
	l      logr.Logger
	cancel context.CancelFunc
}

func (r *imgmgr) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	imageStore := r.ctrlStore.GetImageStore(r.controllerName)
	if imageStore == nil {
		return fmt.Errorf("cannot create img manager, respective controller not initialized in store")
	}
	for _, image := range imageStore.List() {
		de, err := getImageDigestAndEntrypoint(ctx, image.Name)
		if err != nil {
			return err
		}

		podName, err := podName(r.controllerName, image.Name, de.Digest)
		if err != nil {
			return err
		}

		imgc := imgcontroller.New(&imgcontroller.Config{
			Client:         r.client,
			Image:          image,
			PodName:        podName,
			De:             de,
			ConfigMap:      r.cm,
			CreateClientFn: imageStore.SetClient,
			DeleteClientFn: imageStore.DeleteClient,
		})
		go func() {
			if err := imgc.Start(ctx); err != nil {
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

func (r *imgmgr) Stop() {
	r.cancel()
}
