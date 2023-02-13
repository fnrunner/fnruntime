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

package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fnrunner/fnproto/pkg/executor/execclient"
	"github.com/fnrunner/fnproto/pkg/service/svcclient"
	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	"github.com/fnrunner/fnruntime/pkg/fnproxy/imagecontroller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Cache interface {
	List() []fnrunv1alpha1.Image
	Exists(image fnrunv1alpha1.Image) bool
	Create(image fnrunv1alpha1.Image)
	Delete(image fnrunv1alpha1.Image)
	SetConfigMap(image fnrunv1alpha1.Image, cm *corev1.ConfigMap) error
	GetConfigMap(image fnrunv1alpha1.Image) *corev1.ConfigMap
	//SetDigestAndEntrypoint(image fnrunv1alpha1.Image, de *fnrunv1alpha1.DigestAndEntrypoint) error
	//GetDigestAndEntrypoint(image fnrunv1alpha1.Image) *fnrunv1alpha1.DigestAndEntrypoint
	//SetPodName(image fnrunv1alpha1.Image, podName string) error
	//GetPodName(image fnrunv1alpha1.Image) string
	SetClient(image fnrunv1alpha1.Image, IPaddress string) error
	DeleteClient(image fnrunv1alpha1.Image)
	GetFnClient(image fnrunv1alpha1.Image) execclient.Client
	GetSvcClient(image fnrunv1alpha1.Image) svcclient.Client
	//SetWatcher(image fnrunv1alpha1.Image, w watcher.Watcher) error
	//GetWatcher(image fnrunv1alpha1.Image) watcher.Watcher
	Start(ctx context.Context, image fnrunv1alpha1.Image) error
	Stop(ctx context.Context, image fnrunv1alpha1.Image) error
}

func NewCache() Cache {
	return &cache{
		d: map[fnrunv1alpha1.Image]*imageCtx{},
	}
}

type cache struct {
	client *kubernetes.Clientset
	m      sync.RWMutex
	d      map[fnrunv1alpha1.Image]*imageCtx
}

type imageCtx struct {
	imageType  fnrunv1alpha1.ImageKind
	de         *fnrunv1alpha1.DigestAndEntrypoint
	podName    string
	cm         *corev1.ConfigMap
	execclient execclient.Client
	svcclient  svcclient.Client
	cancel     context.CancelFunc
	//watcher    watcher.Watcher
	controller imagecontroller.Controller
}

func (r *cache) List() []fnrunv1alpha1.Image {
	r.m.RLock()
	defer r.m.RUnlock()
	images := make([]fnrunv1alpha1.Image, 0, len(r.d))
	for image := range r.d {
		images = append(images, image)
	}
	return images
}

func (r *cache) Exists(image fnrunv1alpha1.Image) bool {
	r.m.RLock()
	defer r.m.RUnlock()
	if _, ok := r.d[image]; ok {
		return true
	}
	return false
}

func (r *cache) Create(image fnrunv1alpha1.Image) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.d[image]; !ok {
		r.d[image] = &imageCtx{}
	}
	// if the entry already exists we dont want to reinitialize
}

func (r *cache) Delete(image fnrunv1alpha1.Image) {
	r.m.Lock()
	defer r.m.Unlock()
	// cancel the watcher
	if _, ok := r.d[image]; ok {
		if r.d[image].cancel != nil {
			r.d[image].cancel()
		}
	}
	delete(r.d, image)
}

func (r *cache) SetConfigMap(image fnrunv1alpha1.Image, cm *corev1.ConfigMap) error {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.d[image]; !ok {
		return errors.New("cannot set cm, image entry is not initialized")
	}
	r.d[image].cm = cm
	return nil
}

func (r *cache) GetConfigMap(image fnrunv1alpha1.Image) *corev1.ConfigMap {
	r.m.RLock()
	defer r.m.RUnlock()
	c, ok := r.d[image]
	if !ok {
		return nil
	}
	if c.cm == nil {
		return nil
	}
	return c.cm
}

func (r *cache) SetDigestAndEntrypoint(image fnrunv1alpha1.Image, de *fnrunv1alpha1.DigestAndEntrypoint) error {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.d[image]; !ok {
		return errors.New("cannot set digest and entrypoint, image entry is not initialized")
	}
	r.d[image].de = de
	return nil
}

// GetDigestAndEntrypoint returns the digest and entrypoint from the cache
// when the cache is not found or the digest and entrypoint is not initialized an
// empty DigestAndEntrypoint is returned
func (r *cache) GetDigestAndEntrypoint(image fnrunv1alpha1.Image) *fnrunv1alpha1.DigestAndEntrypoint {
	r.m.RLock()
	defer r.m.RUnlock()
	c, ok := r.d[image]
	if !ok {
		return nil
	}
	if c.de == nil {
		return nil
	}
	return c.de
}

func (r *cache) SetPodName(image fnrunv1alpha1.Image, podName string) error {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.d[image]; !ok {
		return errors.New("cannot set podName, image entry is not initialized")
	}
	r.d[image].podName = podName
	return nil
}

func (r *cache) GetPodName(image fnrunv1alpha1.Image) string {
	r.m.RLock()
	defer r.m.RUnlock()
	c, ok := r.d[image]
	if !ok {
		return ""
	}
	return c.podName
}

func (r *cache) SetCancelFn(image fnrunv1alpha1.Image, cancel context.CancelFunc) error {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.d[image]; !ok {
		return errors.New("cannot set podName, image entry is not initialized")
	}
	r.d[image].cancel = cancel
	return nil
}

func (r *cache) GetCancelFn(image fnrunv1alpha1.Image) context.CancelFunc {
	r.m.RLock()
	defer r.m.RUnlock()
	c, ok := r.d[image]
	if !ok {
		return nil
	}
	return c.cancel
}

func (r *cache) SetClient(image fnrunv1alpha1.Image, ipAddr string) error {
	r.m.Lock()
	defer r.m.Unlock()
	c, ok := r.d[image]
	if !ok {
		return errors.New("cannot set client, image entry is not initialized")
	}
	// TBD if we need to deal with updating IP addresses
	address := strings.Join([]string{c.podName, os.Getenv("POD_NAMESPACE"), "svc.cluster.local"}, ".")
	switch image.Kind {
	case fnrunv1alpha1.ImageKindFunction:
		cl, err := execclient.New(&execclient.Config{
			Address: fmt.Sprintf("%s:%d", address, fnrunv1alpha1.FnGRPCServerPort),
			//Address:  fmt.Sprintf("%s:%d", ipAddr, fnrunv1alpha1.FnGRPCServerPort),
			Insecure: true,
		})
		if err != nil {
			return err
		}
		r.d[image].execclient = cl
		return nil
	case fnrunv1alpha1.ImageKindService:
		cl, err := svcclient.New(&svcclient.Config{
			Address: fmt.Sprintf("%s:%d", address, fnrunv1alpha1.FnGRPCServerPort),
			//Address:  fmt.Sprintf("%s:%d", ipAddr, fnrunv1alpha1.FnGRPCServerPort),
			Insecure: true,
		})
		if err != nil {
			return err
		}
		r.d[image].svcclient = cl
		return nil
	default:
		return fmt.Errorf("cannot set client with unknown image kind, got: %s", image.Kind)
	}
}

func (r *cache) DeleteClient(image fnrunv1alpha1.Image) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.d[image]; !ok {
		return
	}
	r.d[image].execclient = nil
	r.d[image].svcclient = nil
}

func (r *cache) GetFnClient(image fnrunv1alpha1.Image) execclient.Client {
	r.m.RLock()
	defer r.m.RUnlock()
	c, ok := r.d[image]
	if !ok {
		return nil
	}
	return c.execclient
}

func (r *cache) GetSvcClient(image fnrunv1alpha1.Image) svcclient.Client {
	r.m.RLock()
	defer r.m.RUnlock()
	c, ok := r.d[image]
	if !ok {
		return nil
	}
	return c.svcclient
}

/*
func (r *cache) SetWatcher(image fnrunv1alpha1.Image, w watcher.Watcher) error {
	r.m.Lock()
	defer r.m.Unlock()
	imgCtx, ok := r.d[image]
	if !ok {
		return errors.New("cannot set watcher, image entry is not initialized")
	}
	imgCtx.watcher = w
	return nil
}

func (r *cache) GetWatcher(image fnrunv1alpha1.Image) watcher.Watcher {
	r.m.RLock()
	defer r.m.RUnlock()
	c, ok := r.d[image]
	if !ok {
		return nil
	}
	return c.watcher
}
*/

func (r *cache) Start(ctx context.Context, image fnrunv1alpha1.Image) error {
	r.m.Lock()
	defer r.m.Unlock()
	imgCtx, ok := r.d[image]
	if !ok {
		return errors.New("cannot start, image entry is not initialized")
	}

	var err error
	imgCtx.de, err = getImageDigestAndEntrypoint(ctx, image.Name)
	if err != nil {
		return err

	}

	imgCtx.podName, err = podName(image.Name, imgCtx.de.Digest)
	if err != nil {
		return err
	}

	imgCtx.controller = imagecontroller.New(&imagecontroller.Config{
		Client:         r.client,
		Image:          image,
		PodName:        imgCtx.podName,
		De:             imgCtx.de,
		ConfigMap:      imgCtx.cm,
		CreateClientFn: r.SetClient,
		DeleteClientFn: r.DeleteClient,
	})
	imgCtx.controller.Start(ctx)
	return nil
}

func (r *cache) Stop(ctx context.Context, image fnrunv1alpha1.Image) error {
	r.m.RLock()
	defer r.m.RUnlock()
	imgCtx, ok := r.d[image]
	if !ok {
		return nil
	}
	if imgCtx.controller != nil {
		return imgCtx.controller.Stop(ctx)
	}
	return nil
}
