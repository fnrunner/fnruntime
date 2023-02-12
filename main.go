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

package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/fnrunner/fnruntime/pkg/ctrlr/controllers/reconciler"
	"github.com/fnrunner/fnruntime/internal/fnproxy/fnproxy"
	"github.com/fnrunner/fnruntime/pkg/ctrlr/fncontroller"

	//"github.com/fnrunner/fnruntime/pkg/ctrlr/manager"
	ctrlcfgv1alpha1 "github.com/fnrunner/fnsyntax/apis/controllerconfig/v1alpha1"
	"github.com/fnrunner/fnsyntax/pkg/ccsyntax"
	"github.com/pkg/profile"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/yaml"
)

/*
const (
	fnImage  = "europe-docker.pkg.dev/srlinux/eu.gcr.io/fn-fabric-image:latest"
	svcImage = "europe-docker.pkg.dev/srlinux/eu.gcr.io/ipam-injector-service-image:latest"
)
*/

// const yamlFile = "./examples/upf.yaml"
const yamlFile = "./examples/topo4.yaml"

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var debug bool
	var profiler bool
	var concurrency int
	var pollInterval time.Duration
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&concurrency, "concurrency", 1, "Number of items to process simultaneously")
	flag.DurationVar(&pollInterval, "poll-interval", 1*time.Minute, "Poll interval controls how often an individual resource should be checked for drift.")
	flag.BoolVar(&debug, "debug", true, "Enable debug")
	flag.BoolVar(&profiler, "profile", false, "Enable profiler")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	l := ctrl.Log.WithName("fnruntime")

	if profiler {
		defer profile.Start().Stop()
		go func() {
			http.ListenAndServe(":8000", nil)
		}()
	}

	mgr, err := manager.New(ctrl.GetConfigOrDie(), manager.Options{
		Scheme:    runtime.NewScheme(),
		Namespace: os.Getenv("POD_NAMESPACE"),
		//MetricsBindAddress: metricsAddr,
		//Port: 9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "c6789sd34.fnrun.io",
	})
	if err != nil {
		l.Error(err, "unable to create manager")
		os.Exit(1)
	}

	fb, err := os.ReadFile(yamlFile)
	if err != nil {
		l.Error(err, "cannot read file")
		os.Exit(1)
	}
	l.Info("read file")

	ctrlcfg := &ctrlcfgv1alpha1.ControllerConfig{}
	if err := yaml.Unmarshal(fb, ctrlcfg); err != nil {
		l.Error(err, "cannot unmarshal")
		os.Exit(1)
	}
	l.Info("unmarshal succeeded", "ctrlcfg", ctrlcfg.Spec.For)

	p, result := ccsyntax.NewParser(ctrlcfg)
	if len(result) > 0 {
		l.Error(err, "ccsyntax validation failed", "result", result)
		os.Exit(1)
	}
	l.Info("ccsyntax validation succeeded")

	ceCtx, result := p.Parse()
	if len(result) != 0 {
		for _, res := range result {
			l.Error(err, "ccsyntax parsing failed", "result", res)
		}
		os.Exit(1)
	}
	l.Info("ccsyntax parsing succeeded")

	gvks, result := p.GetExternalResources()
	if len(result) > 0 {
		l.Error(err, "ccsyntax get external resources failed", "result", result)
		os.Exit(1)
	}
	// validate if we can resolve the gvr to gvk in the system
	for _, gvk := range gvks {
		gvk, err := mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
		if err != nil {
			l.Error(err, "ccsyntax get gvk mapping in api server", "result", result)
			os.Exit(1)
		}
		l.Info("gvk", "value", gvk)
	}

	ge := make(chan event.GenericEvent)

	l.Info("setup fnruntime controller")
	ctx := ctrl.SetupSignalHandler()

	fnc := fncontroller.New(mgr, ceCtx, ge)
	fnc.Start(ctx, ctrlcfg.Name, controller.Options{
		Reconciler: reconciler.New(&reconciler.Config{
			Client:       mgr.GetClient(),
			PollInterval: 1 * time.Minute,
			CeCtx:        ceCtx,
		}),
	},
	)

	c, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		l.Error(err, "unable to create clientset")
		os.Exit(1)
	}

	fnProxy := fnproxy.New(&fnproxy.Config{
		Clientset: c,
	})
	go fnProxy.Start(ctx)

	// create the fn pods based on the image information
	for _, image := range p.GetImages() {
		if err := fnProxy.CreatePod(ctx, *image); err != nil {
			l.Error(err, "unable to create svc pod")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		l.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		l.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	l.Info("starting controller manager")
	if err := mgr.Start(ctx); err != nil {
		l.Error(err, "cannot run manager")
		os.Exit(1)
	}
}
