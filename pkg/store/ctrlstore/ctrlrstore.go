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

package ctrlstore

import (
	"fmt"
	"sync"

	"github.com/fnrunner/fnruntime/pkg/store/imagestore"
	corev1 "k8s.io/api/core/v1"
)

type Store interface {
	List() []string
	Exists(controllerName string) bool
	Create(controllerName string)
	Delete(controllerName string)
	SetConfigMap(controllerName string, cm *corev1.ConfigMap) error
	GetConfigMap(controllerName string) *corev1.ConfigMap

	GetImageStore(controllerName string) imagestore.Store
}

func New() Store {
	return &store{
		d: map[string]*controllerCtx{},
	}
}

type store struct {
	m sync.RWMutex
	d map[string]*controllerCtx
}

type controllerCtx struct {
	configMap  *corev1.ConfigMap
	imageStore imagestore.Store
}

func (r *store) List() []string {
	r.m.RLock()
	defer r.m.RUnlock()
	controllers := make([]string, 0, len(r.d))
	for controller := range r.d {
		controllers = append(controllers, controller)
	}
	return controllers
}

func (r *store) Exists(controllerName string) bool {
	r.m.RLock()
	defer r.m.RUnlock()
	if _, ok := r.d[controllerName]; ok {
		return true
	}
	return false
}

func (r *store) Create(controllerName string) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.d[controllerName]; !ok {
		r.d[controllerName] = &controllerCtx{
			imageStore: imagestore.New(),
		}
	}
	// if the entry already exists we dont want to reinitialize
}

func (r *store) Delete(controllerName string) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.d, controllerName)
}

func (r *store) SetConfigMap(controllerName string, cm *corev1.ConfigMap) error {
	r.m.Lock()
	defer r.m.Unlock()
	ctrlCtx, ok := r.d[controllerName]
	if !ok {
		return fmt.Errorf("cannot set configMap, controller entry is not initialized")
	}
	ctrlCtx.configMap = cm
	return nil
}

func (r *store) GetConfigMap(controllerName string) *corev1.ConfigMap {
	r.m.RLock()
	defer r.m.RUnlock()
	ctrlCtx, ok := r.d[controllerName]
	if ok {
		return ctrlCtx.configMap
	}
	return nil
}

func (r *store) GetImageStore(controllerName string) imagestore.Store {
	r.m.RLock()
	defer r.m.RUnlock()
	ctrlCtx, ok := r.d[controllerName]
	if ok {
		return ctrlCtx.imageStore
	}
	return nil
}
