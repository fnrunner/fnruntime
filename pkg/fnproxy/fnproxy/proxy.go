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

package fnproxy

import (
	"context"
	"fmt"

	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	"github.com/fnrunner/fnruntime/pkg/fnproxy/exechandler"
	"github.com/fnrunner/fnruntime/pkg/fnproxy/grpcserver"
	"github.com/fnrunner/fnruntime/pkg/fnproxy/healthhandler"
	"github.com/fnrunner/fnruntime/pkg/fnproxy/servicehandler"
	"github.com/fnrunner/fnruntime/pkg/store/ctrlstore"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Config struct {
	ControllerStore ctrlstore.Store
	//Clientset      *kubernetes.Clientset
	//FnWrapperImage string
	//Images         []*fnrunv1alpha1.Image
	//ConfigMap      *corev1.ConfigMap
}

type Proxy interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context)
	//CreatePod(ctx context.Context, image fnrunv1alpha1.Image) error
	//DeletePod(ctx context.Context, image fnrunv1alpha1.Image) error
}

func New(cfg *Config) Proxy {
	l := ctrl.Log.WithName("fn proxy")

	/*
		namespace := os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = "default"
		}

		fnWrapperImage := os.Getenv(fnrunv1alpha1.EnvFnWrapperImage)
		if fnWrapperImage == "" {
			fnWrapperImage = fnrunv1alpha1.DefaultFnWrapperImage
		}
	*/

	/*
		c := cache.NewCache(cfg.Clientset)
		for _, image := range cfg.Images {
			c.Create(*image)
			// since we created the image here we dont have to validate the error
			// when setting the configmap
			c.SetConfigMap(*image, cfg.ConfigMap)
		}
	*/

	hh := healthhandler.New()
	sh := servicehandler.New(cfg.ControllerStore)
	eh := exechandler.New(cfg.ControllerStore)

	s := grpcserver.New(grpcserver.Config{
		Address:  fmt.Sprintf(":%d", fnrunv1alpha1.FnProxyGRPCServerPort),
		Insecure: true,
	},
		grpcserver.WithServiceApplyResourceHandler(sh.ApplyResource),
		grpcserver.WithServiceDeleteResourceHandler(sh.DeleteResource),
		grpcserver.WithExecHandler(eh.ExecuteFuntion),
		grpcserver.WithWatchHandler(hh.Watch),
		grpcserver.WithCheckHandler(hh.Check),
	)

	return &proxy{
		s: s,
		//clientset:      cfg.Clientset,
		//namespace:      namespace,
		//fnWrapperImage: fnWrapperImage,
		ctrlStore: cfg.ControllerStore,
		l:         l,
		//cm:             cfg.ConfigMap,
	}
}

type proxy struct {
	s *grpcserver.GrpcServer
	//clientset      *kubernetes.Clientset
	//namespace      string
	//fnWrapperImage string
	ctrlStore ctrlstore.Store
	l         logr.Logger
	//cm             *corev1.ConfigMap
	cancel context.CancelFunc
}

func (r *proxy) Stop(ctx context.Context) {
	// delete pods/svcs -> happens through ownerreference
	/*
		for _, image := range r.cache.List() {
			if err := r.cache.Stop(ctx, image); err != nil {
				return err
			}
		}
	*/
	// stop grpc and imager controller servers
	r.cancel()
	//return nil
}

func (r *proxy) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	if err := r.s.Start(ctx); err != nil {
		r.l.Error(err, "cannot start grpcserver")
		return err
	}

	// create pods/svc
	/*
		for _, image := range r.cache.List() {
			if err := r.cache.Start(ctx, image); err != nil {
				r.l.Error(err, "cannot start image controller")
				return err
			}
		}
	*/

	return nil
}

/*
func (r *proxy) DeletePod(ctx context.Context, image fnrunv1alpha1.Image) error {
	r.l.WithValues("image", image)

	if r.cache.Exists(image) {
		w := r.cache.GetWatcher(image)
		if w != nil {
			w.Stop()
		}

		podName := r.cache.GetPodName(image)
		if err := r.clientset.CoreV1().Pods(r.namespace).Delete(ctx, podName, metav1.DeleteOptions{}); err != nil {
			r.l.Error(err, "cannot create image")
			return err
		}
		if err := r.clientset.CoreV1().Services(r.namespace).Delete(ctx, podName, metav1.DeleteOptions{}); err != nil {
			return err
		}
		// delete image from the cache
		r.cache.Delete(image)
	}

	return nil
}

func (r *proxy) CreatePod(ctx context.Context, image fnrunv1alpha1.Image) error {
	r.l.WithValues("image", image)
	// add image to the cache
	r.cache.Create(image)

	podKey, err := r.getOrCreatePod(ctx, image)
	if err != nil {
		r.l.Error(err, "cannot get or create pod")
		return err
	}

	w := watcher.New(&watcher.Config{
		ClientSet:      r.clientset,
		Image:          image,
		Namespace:      r.namespace,
		PodName:        podKey.Name,
		DeleteClientFn: r.deleteClient,
		CreateClientFn: r.createClient,
	})

	if err := r.cache.SetWatcher(image, w); err != nil {
		r.l.Error(err, "cannot set watcher")
		return err
	}
	go w.Start(ctx)
	return nil
}

func (r *proxy) getOrCreatePod(ctx context.Context, image fnrunv1alpha1.Image) (types.NamespacedName, error) {
	r.l.WithValues("image", image)
	de, err := getImageDigestAndEntrypoint(ctx, image.Name)
	if err != nil {
		r.l.Error(err, "cannot get image digest and entrypoint")
		return types.NamespacedName{}, err
	}
	r.cache.SetDigestAndEntrypoint(image, de)

	podName, err := podName(image.Name, de.Digest)
	if err != nil {
		r.l.Error(err, "cannot get podName")
		return types.NamespacedName{}, err
	}
	r.cache.SetPodName(image, podName)

	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			fnrunv1alpha1.FunctionLabelKey: podName,
		}}
	podList, err := r.clientset.CoreV1().Pods(r.namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		})
	if err != nil {
		r.l.Error(err, "cannot list pods")
		return types.NamespacedName{}, err
	}
	// check if pod exists
	if len(podList.Items) > 0 {
		for _, pod := range podList.Items {
			if pod.DeletionTimestamp == nil {
				r.l.Info("fn executor pod already exists")
				return client.ObjectKeyFromObject(&pod), nil
			}
		}
	}
	svc := r.buildService(image, podName)
	if _, err := r.clientset.CoreV1().Services(r.namespace).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
		if meta.IgnoreAlreadyExists(err) != nil {
			return types.NamespacedName{}, err
		}
	}
	pod, err := r.buildPod(image, podName)
	if err != nil {
		return types.NamespacedName{}, err
	}
	if _, err := r.clientset.CoreV1().Pods(r.namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		if meta.IgnoreAlreadyExists(err) != nil {
			return types.NamespacedName{}, err
		}
	}
	return client.ObjectKeyFromObject(pod), nil
}

func (r *proxy) buildPod(image fnrunv1alpha1.Image, podName string) (*corev1.Pod, error) {
	switch image.Kind {
	case fnrunv1alpha1.ImageKindFunction:
		return r.buildFnPod(image, podName)
	case fnrunv1alpha1.ImageKindService:
		return r.buildSvcPod(image, podName)
	default:
		return nil, fmt.Errorf("cannot build pod with unknown image kind, got: %s", image.Kind)
	}
}

func (r *proxy) buildFnPod(image fnrunv1alpha1.Image, podName string) (*corev1.Pod, error) {
	de := r.cache.GetDigestAndEntrypoint(image)
	if de == nil {
		return nil, errors.New("cannot return pod since digest is not initialized")
	}
	cmd := append([]string{
		filepath.Join(fnrunv1alpha1.VolumeMountPath, fnrunv1alpha1.WrapperServerBin),
		"--port", strconv.Itoa(fnwrapper.FnGRPCServerPort), "--",
	}, de.GetEntrypoint()...)

	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.namespace,
			Name:      podName,
			Labels: map[string]string{
				fnrunv1alpha1.FunctionLabelKey: podName,
			},
			OwnerReferences: []metav1.OwnerReference{},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:  fnrunv1alpha1.InitContainerName,
					Image: r.fnWrapperImage,
					Command: []string{
						"cp",
						"-a",
						"/" + fnrunv1alpha1.WrapperServerBin + "/.",
						fnrunv1alpha1.VolumeMountPath,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      fnrunv1alpha1.VolumeName,
							MountPath: fnrunv1alpha1.VolumeMountPath,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    fnrunv1alpha1.FnContainerName,
					Image:   image.Name,
					Command: cmd,
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							GRPC: &corev1.GRPCAction{
								Port:    fnwrapper.FnGRPCServerPort,
								Service: pointer.String(podName),
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      fnrunv1alpha1.VolumeName,
							MountPath: fnrunv1alpha1.VolumeMountPath,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: fnrunv1alpha1.VolumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}, nil
}

func (r *proxy) buildSvcPod(image fnrunv1alpha1.Image, podName string) (*corev1.Pod, error) {
	de := r.cache.GetDigestAndEntrypoint(image)
	if de == nil {
		return nil, errors.New("cannot return pod since digest is not initialized")
	}
	cmd := de.GetEntrypoint()

	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.namespace,
			Name:      podName,
			Labels: map[string]string{
				fnrunv1alpha1.FunctionLabelKey: podName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    fnrunv1alpha1.FnContainerName,
					Image:   image.Name,
					Command: cmd,
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							GRPC: &corev1.GRPCAction{
								Port:    fnwrapper.FnGRPCServerPort,
								Service: pointer.String(podName),
							},
						},
					},
				},
			},
		},
	}, nil
}

func (r *proxy) buildService(image fnrunv1alpha1.Image, podName string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.namespace,
			Name:      podName,
			Labels: map[string]string{
				fnrunv1alpha1.FunctionLabelKey: podName,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				fnrunv1alpha1.FunctionLabelKey: podName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Port:       fnwrapper.FnGRPCServerPort,
					TargetPort: intstr.FromInt(fnwrapper.FnGRPCServerPort),
					Protocol:   corev1.Protocol("TCP"),
				},
			},
			ClusterIP: "None",
		},
	}
}

func (r *proxy) deleteClient(image fnrunv1alpha1.Image) {
	r.cache.DeleteClient(image)
}

func (r *proxy) createClient(image fnrunv1alpha1.Image, podIP string) {
	r.cache.SetClient(image, podIP)
}
*/
