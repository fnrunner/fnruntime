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

package imagestore

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fnrunner/fnproto/pkg/executor/execclient"
	"github.com/fnrunner/fnproto/pkg/service/svcclient"
	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
)

type Store interface {
	List() []fnrunv1alpha1.Image
	Exists(image fnrunv1alpha1.Image) bool
	Create(image fnrunv1alpha1.Image)
	Delete(image fnrunv1alpha1.Image)
	//SetConfigMap(image fnrunv1alpha1.Image, cm *corev1.ConfigMap) error
	//GetConfigMap(image fnrunv1alpha1.Image) *corev1.ConfigMap
	SetClient(image fnrunv1alpha1.Image, podName, IPaddress string) error
	DeleteClient(image fnrunv1alpha1.Image)
	GetFnClient(image fnrunv1alpha1.Image) execclient.Client
	GetSvcClient(image fnrunv1alpha1.Image) svcclient.Client
}

func New() Store {
	return &store{
		d: map[fnrunv1alpha1.Image]*imageCtx{},
	}
}

type store struct {
	m sync.RWMutex
	d map[fnrunv1alpha1.Image]*imageCtx
}

type imageCtx struct {
	imageType fnrunv1alpha1.ImageKind
	//de         *fnrunv1alpha1.DigestAndEntrypoint
	//podName    string
	//cm         *corev1.ConfigMap
	execclient execclient.Client
	svcclient  svcclient.Client
}

func (r *store) List() []fnrunv1alpha1.Image {
	r.m.RLock()
	defer r.m.RUnlock()
	images := make([]fnrunv1alpha1.Image, 0, len(r.d))
	for image := range r.d {
		images = append(images, image)
	}
	return images
}

func (r *store) Exists(image fnrunv1alpha1.Image) bool {
	r.m.RLock()
	defer r.m.RUnlock()
	if _, ok := r.d[image]; ok {
		return true
	}
	return false
}

func (r *store) Create(image fnrunv1alpha1.Image) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.d[image]; !ok {
		r.d[image] = &imageCtx{}
	}

	// if the entry already exists we dont want to reinitialize
}

func (r *store) Delete(image fnrunv1alpha1.Image) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.d, image)
}

func (r *store) SetClient(image fnrunv1alpha1.Image, podName, ipAddr string) error {
	r.m.Lock()
	defer r.m.Unlock()
	// TBD if we need to deal with updating IP addresses
	address := strings.Join([]string{podName, os.Getenv("POD_NAMESPACE"), "svc.cluster.local"}, ".")
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

func (r *store) DeleteClient(image fnrunv1alpha1.Image) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.d[image]; !ok {
		return
	}
	r.d[image].execclient = nil
	r.d[image].svcclient = nil
}

func (r *store) GetFnClient(image fnrunv1alpha1.Image) execclient.Client {
	r.m.RLock()
	defer r.m.RUnlock()
	c, ok := r.d[image]
	if !ok {
		return nil
	}
	return c.execclient
}

func (r *store) GetSvcClient(image fnrunv1alpha1.Image) svcclient.Client {
	r.m.RLock()
	defer r.m.RUnlock()
	c, ok := r.d[image]
	if !ok {
		return nil
	}
	return c.svcclient
}
