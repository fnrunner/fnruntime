package imagecontroller

import (
	"context"
	"fmt"
	"os"
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
	defaultNamespace = "default"
	defaultWaitTime  = time.Second * 5
	finalizer        = "fnrun.io/finalizer"
)

type CreateClientFn func(image fnrunv1alpha1.Image, podIP string) error
type DeleteCLientFn func(image fnrunv1alpha1.Image)

type Controller interface {
	Start(ctx context.Context)
	Stop(ctx context.Context) error
}

type Config struct {
	Client         *kubernetes.Clientset
	Image          fnrunv1alpha1.Image
	PodName        string
	De             *fnrunv1alpha1.DigestAndEntrypoint
	ConfigMap      *corev1.ConfigMap
	CreateClientFn CreateClientFn
	DeleteClientFn DeleteCLientFn
}

func New(cfg *Config) Controller {
	l := ctrl.Log.WithName("configmap controller").WithValues(
		"image", cfg.Image,
	)
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = defaultNamespace
	}
	fnWrapperImage := os.Getenv(fnrunv1alpha1.EnvFnWrapperImage)
	if fnWrapperImage == "" {
		fnWrapperImage = fnrunv1alpha1.DefaultFnWrapperImage
	}

	return &controller{
		client:         cfg.Client,
		namespace:      namespace,
		image:          cfg.Image,
		fnWrapperImage: fnWrapperImage,
		cm:             cfg.ConfigMap,
		l:              l,
		createClientFn: cfg.CreateClientFn,
		deleteClientFn: cfg.DeleteClientFn,
		de:             cfg.De,
		podName:        cfg.PodName,
	}
}

type controller struct {
	client         *kubernetes.Clientset
	podName        string
	namespace      string
	image          fnrunv1alpha1.Image
	fnWrapperImage string
	cm             *corev1.ConfigMap
	l              logr.Logger
	createClientFn CreateClientFn
	deleteClientFn DeleteCLientFn
	de             *fnrunv1alpha1.DigestAndEntrypoint
}

func (r *controller) Stop(ctx context.Context) error {
	if err := r.deletePod(ctx, r.podName); err != nil {
		r.l.Error(err, "cannot delete pod")
		return err
	}
	if err := r.deleteService(ctx, r.podName); err != nil {
		r.l.Error(err, "cannot delete service")
		return err
	}
	return nil
}

func (r *controller) Start(ctx context.Context) {
	for {
		select {
		default:
		INIT:
			if _, err := r.applyPod(ctx, r.podName); err != nil {
				r.l.Error(err, "cannot apply pod")
				time.Sleep(defaultWaitTime)
				goto INIT
			}
			if _, err := r.applyService(ctx, r.podName); err != nil {
				r.l.Error(err, "cannot apply service")
				time.Sleep(defaultWaitTime)
				goto INIT
			}

			if err := r.start(ctx, r.podName); err != nil {
				//r.l.Error(err, "watch failed")
				time.Sleep(defaultWaitTime)
				goto INIT
			}
			//goto INIT
		case <-ctx.Done():
			return
		}
	}
}

func (r *controller) start(ctx context.Context, podName string) error {
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			fnrunv1alpha1.FunctionLabelKey: podName,
		}}
	wpi, err := r.client.CoreV1().Pods(r.namespace).Watch(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
			Watch:         true,
		})
	if err != nil {
		r.l.Error(err, "cannot create pod watch")
		return err
	}
	wsi, err := r.client.CoreV1().Services(r.namespace).Watch(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
			Watch:         true,
		})
	if err != nil {
		r.l.Error(err, "cannot create service watch")
		return err
	}

	for {
		select {
		case we, ok := <-wpi.ResultChan():
			if !ok {
				err := fmt.Errorf("watch result nok, we: %v", we)
				r.l.Error(err, "cannot watch pod channel")
				return err
			}
			r.l.Info("pod event", "event", we)
			if _, err := r.applyPod(ctx, podName); err != nil {
				r.l.Error(err, "cannot apply")
			}
		case we, ok := <-wsi.ResultChan():
			if !ok {
				err := fmt.Errorf("watch result nok, we: %v", we)
				r.l.Error(err, "cannot watch svc channel")
				return err
			}
			r.l.Info("service event", "event", we)
			if _, err := r.applyService(ctx, podName); err != nil {
				r.l.Error(err, "cannot apply")
			}

		case <-ctx.Done():
			return nil
		}
	}
}
