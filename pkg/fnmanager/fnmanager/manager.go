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

package fnmanager

import (
	"context"
	"fmt"
	"os"
	"time"

	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	"github.com/fnrunner/fnruntime/pkg/fnmanager/fnctrlrmanager"
	"github.com/fnrunner/fnruntime/pkg/fnproxy/fnproxy"
	"github.com/fnrunner/fnruntime/pkg/store/ctrlstore"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Manager interface {
	Start(ctx context.Context) error
}

type Config struct {
	Domain               string
	UniqueID             string   // need to come at init time
	ConfigMaps           []string // need to come at init time
	MetricAddress        string
	ProbeAddress         string
	EnableLeaderElection bool
	Concurrency          int
	PollInterval         time.Duration
}

func New(cfg *Config) (Manager, error) {
	l := ctrl.Log.WithName("fn manager")
	var err error
	fnmgr, err := initDefaults(cfg)
	if err != nil {
		return nil, err
	}
	fnmgr.errChan = make(chan error)

	fnmgr.mgr, err = manager.New(ctrl.GetConfigOrDie(), manager.Options{
		Scheme:    runtime.NewScheme(),
		Namespace: fnmgr.namespace,
		//MetricsBindAddress: metricsAddr,
		//Port: 9443,
		HealthProbeBindAddress: fnmgr.probeAddr,
		LeaderElection:         fnmgr.leaderElection,
		LeaderElectionID:       fnmgr.leaderElectionID,
	})
	if err != nil {
		return nil, err
	}

	fnmgr.client, err = kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		l.Error(err, "unable to create clientset")
		os.Exit(1)
	}

	// create controller store
	fnmgr.ctrlStore = ctrlstore.New()
	for _, controllerName := range fnmgr.configMaps {
		fnmgr.ctrlStore.Create(controllerName)
	}
	// create fn controller manager
	fnmgr.fncm = fnctrlrmanager.New(&fnctrlrmanager.Config{
		ControllerStore: fnmgr.ctrlStore,
		Client:          fnmgr.client,
		Namespace:       fnmgr.namespace,
		Manager:         fnmgr.mgr,
	})

	fnmgr.proxy = fnproxy.New(&fnproxy.Config{
		ControllerStore: fnmgr.ctrlStore,
	})

	// add health/ready checks
	if err := fnmgr.mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		l.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := fnmgr.mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		l.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	return fnmgr, nil
}

type fnmgr struct {
	errChan chan error

	namespace        string
	uniqueId         string
	configMaps       []string
	metricsAddr      string
	probeAddr        string
	leaderElection   bool
	leaderElectionID string
	concurrency      int
	pollInterval     time.Duration

	client    *kubernetes.Clientset
	ctrlStore ctrlstore.Store
	mgr       manager.Manager
	fncm      fnctrlrmanager.Manager
	proxy     fnproxy.Proxy
	l         logr.Logger
}

func initDefaults(cfg *Config) (*fnmgr, error) {
	if cfg.UniqueID == "" {
		return nil, fmt.Errorf("cannot run fn manager, unique id is required")
	}
	if len(cfg.ConfigMaps) == 0 {
		return nil, fmt.Errorf("cannot run fn manager, a controller configmap is required")
	}

	fnmgr := &fnmgr{
		uniqueId:   cfg.UniqueID,
		configMaps: cfg.ConfigMaps,
	}
	fnmgr.namespace = os.Getenv("POD_NAMESPACE")
	if fnmgr.namespace == "" {
		fnmgr.namespace = "default"
	}
	fnmgr.metricsAddr = cfg.MetricAddress
	if fnmgr.metricsAddr == "" {
		fnmgr.metricsAddr = ":8080"
	}
	fnmgr.probeAddr = cfg.ProbeAddress
	if fnmgr.probeAddr == "" {
		fnmgr.metricsAddr = ":8081"
	}
	fnmgr.leaderElection = cfg.EnableLeaderElection
	domain := cfg.Domain
	if domain == "" {
		domain = fnrunv1alpha1.Domain
	}
	fnmgr.leaderElectionID = fmt.Sprintf("%s.%s", cfg.UniqueID, domain)

	fnmgr.concurrency = cfg.Concurrency
	if fnmgr.concurrency == 0 {
		fnmgr.concurrency = 1
	}
	fnmgr.pollInterval = cfg.PollInterval

	return fnmgr, nil
}

func (r *fnmgr) Start(ctx context.Context) error {
	// start the proxy
	go func() {
		if err := r.proxy.Start(ctx); err != nil {
			r.errChan <- err
		}
	}()
	// start the fn controller manager
	go func() {
		if err := r.fncm.Start(ctx); err != nil {
			r.errChan <- err
		}
	}()
	go func() {
		if err := r.mgr.Start(ctx); err != nil {
			r.errChan <- err
		}
	}()
	// block until cancelled or err occurs
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
