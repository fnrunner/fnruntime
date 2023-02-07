package service

import (
	"sync"

	ctrlcfgv1alpha1 "github.com/fnrunner/fnsyntax/apis/controllerconfig/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Services interface {
	AddEntry(k schema.GroupVersionKind, v ServiceCtx)
	Add(Services)
	Get() map[schema.GroupVersionKind]ServiceCtx
	GetValue(schema.GroupVersionKind) ServiceCtx
	Length() int
}

func New() Services {
	return &service{
		d: map[schema.GroupVersionKind]ServiceCtx{},
	}
}

type service struct {
	m sync.RWMutex
	d map[schema.GroupVersionKind]ServiceCtx
}

type ServiceCtx struct {
	Port int
	Fn   ctrlcfgv1alpha1.Function
	//Client fnservicepb.ServiceFunctionClient
}

func (r *service) AddEntry(k schema.GroupVersionKind, v ServiceCtx) {
	r.m.Lock()
	defer r.m.Unlock()
	r.d[k] = v
}

func (r *service) Add(o Services) {
	r.m.Lock()
	defer r.m.Unlock()
	for k, v := range o.Get() {
		r.d[k] = v
	}
}

func (r *service) Get() map[schema.GroupVersionKind]ServiceCtx {
	r.m.RLock()
	defer r.m.RUnlock()
	d := make(map[schema.GroupVersionKind]ServiceCtx, len(r.d))
	for k, v := range r.d {
		d[k] = v
	}
	return d
}

func (r *service) GetValue(k schema.GroupVersionKind) ServiceCtx {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.d[k]
}

func (r *service) Length() int {
	r.m.RLock()
	defer r.m.RUnlock()
	return len(r.d)
}
