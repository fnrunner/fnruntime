package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	"github.com/fnrunner/fnruntime/internal/podproxy"
	"github.com/pkg/profile"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	fnImage  = "europe-docker.pkg.dev/srlinux/eu.gcr.io/fn-fabric-image"
	svcImage = "europe-docker.pkg.dev/srlinux/eu.gcr.io/fn-ipam-service-image:latest"
)

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
		Namespace:              os.Getenv("POD_NAMESPACE"),
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		l.Error(err, "unable to create manager")
		os.Exit(1)
	}

	l.Info("setup controller")
	ctx := ctrl.SetupSignalHandler()

	c, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		l.Error(err, "unable to create clientset")
		os.Exit(1)
	}

	if err := podproxy.New(&podproxy.Config{
		Clientset: c,
	}).CreatePod(ctx, fnrunv1alpha1.Image{Name: svcImage, Kind: fnrunv1alpha1.ImageKindService}); err != nil {
		l.Error(err, "unable to create pod")
		os.Exit(1)
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
