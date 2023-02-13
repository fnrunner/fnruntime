package imagecontroller

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreapplyv1 "k8s.io/client-go/applyconfigurations/core/v1"
)

func (r *controller) applyPod(ctx context.Context, podName string) (*corev1.Pod, error) {
	// apply pod
	pod, err := r.client.CoreV1().Pods(r.namespace).Apply(ctx, r.buildPod(r.image, podName), metav1.ApplyOptions{FieldManager: "application/apply-patch"})
	if err != nil {
		return nil, err
	}

	pod, err = r.client.CoreV1().Pods(r.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		r.deleteClientFn(r.image)
		return nil, err
	}
	if pod.Status.Phase != "Running" {
		r.deleteClientFn(r.image)
		return pod, nil
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			r.createClientFn(r.image, pod.Status.PodIP)
			return pod, nil
		}
		r.deleteClientFn(r.image)
	}
	return pod, nil
}

func (r *controller) deletePod(ctx context.Context, podName string) error {
	// delete pod
	return r.client.CoreV1().Pods(r.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

func (r *controller) buildPod(image fnrunv1alpha1.Image, podName string) *coreapplyv1.PodApplyConfiguration {
	pod := &coreapplyv1.PodApplyConfiguration{}
	pod.WithAPIVersion("v1")
	pod.WithKind("Pod")
	pod.WithNamespace(r.namespace)
	pod.WithName(podName)
	pod.WithLabels(map[string]string{
		fnrunv1alpha1.FunctionLabelKey: podName,
	})
	pod.WithOwnerReferences() // TODO

	// probe
	probe := &coreapplyv1.ProbeApplyConfiguration{}
	grpc := &coreapplyv1.GRPCActionApplyConfiguration{}
	grpc.WithPort(fnrunv1alpha1.FnGRPCServerPort)
	grpc.WithService(podName)
	probe.WithGRPC(grpc)

	switch image.Kind {
	case fnrunv1alpha1.ImageKindFunction:
		// init container
		initContainer := &coreapplyv1.ContainerApplyConfiguration{}
		initContainer.WithName(fnrunv1alpha1.InitContainerName)
		initContainer.WithImage(r.fnWrapperImage)
		initContainer.WithCommand([]string{
			"cp",
			"-a",
			"/" + fnrunv1alpha1.WrapperServerBin + "/.",
			fnrunv1alpha1.VolumeMountPath,
		}...)
		initContainerVolumeMount := &coreapplyv1.VolumeMountApplyConfiguration{}
		initContainerVolumeMount.WithName(fnrunv1alpha1.VolumeName)
		initContainerVolumeMount.WithMountPath(fnrunv1alpha1.VolumeMountPath)
		initContainer.WithVolumeMounts(initContainerVolumeMount)
		// container
		cmd := append([]string{
			filepath.Join(fnrunv1alpha1.VolumeMountPath, fnrunv1alpha1.WrapperServerBin),
			"--port", strconv.Itoa(fnrunv1alpha1.FnGRPCServerPort), "--",
		}, r.de.GetEntrypoint()...)
		container := &coreapplyv1.ContainerApplyConfiguration{}
		container.WithName(fnrunv1alpha1.FnContainerName)
		container.WithImage(r.image.Name)
		container.WithCommand(cmd...)
		containerVolumeMount := &coreapplyv1.VolumeMountApplyConfiguration{}
		containerVolumeMount.WithName(fnrunv1alpha1.VolumeName)
		containerVolumeMount.WithMountPath(fnrunv1alpha1.VolumeMountPath)
		container.WithVolumeMounts(initContainerVolumeMount)
		container.WithReadinessProbe(probe)

		volume := &coreapplyv1.VolumeApplyConfiguration{}
		volume.WithName(fnrunv1alpha1.VolumeName)
		volume.WithEmptyDir(&coreapplyv1.EmptyDirVolumeSourceApplyConfiguration{})

		podSpec := &coreapplyv1.PodSpecApplyConfiguration{}
		podSpec.WithInitContainers(initContainer)
		podSpec.WithContainers(container)
		podSpec.WithVolumes(volume)

		pod.WithSpec(podSpec)
		return pod
	case fnrunv1alpha1.ImageKindService:
		// container
		cmd := r.de.GetEntrypoint()

		container := &coreapplyv1.ContainerApplyConfiguration{}
		container.WithName(fnrunv1alpha1.FnContainerName)
		container.WithImage(r.image.Name)
		container.WithCommand(cmd...)
		container.WithReadinessProbe(probe)

		podSpec := &coreapplyv1.PodSpecApplyConfiguration{}
		podSpec.WithContainers(container)

		pod.WithSpec(podSpec)
		return pod
	default:
		err := fmt.Errorf("cannot build pod with unknown image kind, got: %s", image.Kind)
		r.l.Error(err, "unknown image kind")
		return &coreapplyv1.PodApplyConfiguration{}
	}
}

/*
func (r *controller) buildFnPod(image fnrunv1alpha1.Image, podName string) (*coreapplyv1.PodApplyConfiguration, error) {
	pod := &coreapplyv1.PodApplyConfiguration{}
	pod.WithAPIVersion("v1")
	pod.WithKind("Pod")
	pod.WithNamespace("r.namespace")
	pod.WithName(podName)
	pod.WithLabels(map[string]string{
		fnrunv1alpha1.FunctionLabelKey: podName,
	})
	pod.WithOwnerReferences() // TODO

	// init container
	initContainer := &coreapplyv1.ContainerApplyConfiguration{}
	initContainer.WithName(fnrunv1alpha1.InitContainerName)
	initContainer.WithImage(r.fnWrapperImage)
	initContainer.WithCommand([]string{
		"cp",
		"-a",
		"/" + fnrunv1alpha1.WrapperServerBin + "/.",
		fnrunv1alpha1.VolumeMountPath,
	}...)
	initContainerVolumeMount := &coreapplyv1.VolumeMountApplyConfiguration{}
	initContainerVolumeMount.WithName(fnrunv1alpha1.VolumeName)
	initContainerVolumeMount.WithMountPath(fnrunv1alpha1.VolumeMountPath)
	initContainer.WithVolumeMounts(initContainerVolumeMount)

	// container probe
	probe := &coreapplyv1.ProbeApplyConfiguration{}
	grpc := &coreapplyv1.GRPCActionApplyConfiguration{}
	grpc.WithPort(fnrunv1alpha1.FnGRPCServerPort)
	grpc.WithService(podName)
	probe.WithGRPC(grpc)
	// container
	de := r.cache.GetDigestAndEntrypoint(image)
	if de == nil {
		return nil, errors.New("cannot return pod since digest is not initialized")
	}
	cmd := append([]string{
		filepath.Join(fnrunv1alpha1.VolumeMountPath, fnrunv1alpha1.WrapperServerBin),
		"--port", strconv.Itoa(fnrunv1alpha1.FnGRPCServerPort), "--",
	}, de.GetEntrypoint()...)
	container := &coreapplyv1.ContainerApplyConfiguration{}
	container.WithName(fnrunv1alpha1.FnContainerName)
	container.WithImage(r.image.Name)
	container.WithCommand(cmd...)
	containerVolumeMount := &coreapplyv1.VolumeMountApplyConfiguration{}
	containerVolumeMount.WithName(fnrunv1alpha1.VolumeName)
	containerVolumeMount.WithMountPath(fnrunv1alpha1.VolumeMountPath)
	container.WithVolumeMounts(initContainerVolumeMount)
	container.WithReadinessProbe(probe)

	volume := &coreapplyv1.VolumeApplyConfiguration{}
	volume.WithName(fnrunv1alpha1.VolumeName)
	volume.WithEmptyDir(&coreapplyv1.EmptyDirVolumeSourceApplyConfiguration{})

	podSpec := &coreapplyv1.PodSpecApplyConfiguration{}
	podSpec.WithInitContainers(initContainer)
	podSpec.WithContainers(container)
	podSpec.WithVolumes(volume)

	pod.WithSpec(podSpec)

	return pod, nil

		return &coreapplyv1.PodApplyConfiguration{
			TypeMeta: metav1.TypeMetaApplyConfiguration{
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
									Port:    fnrunv1alpha1.FnGRPCServerPort,
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
*/
/*
func (r *controller) buildSvcPod(image fnrunv1alpha1.Image, podName string) (*coreapplyv1.PodApplyConfiguration, error) {
	pod := &coreapplyv1.PodApplyConfiguration{}
	pod.WithAPIVersion("v1")
	pod.WithKind("Pod")
	pod.WithNamespace("r.namespace")
	pod.WithName(podName)
	pod.WithLabels(map[string]string{
		fnrunv1alpha1.FunctionLabelKey: podName,
	})
	pod.WithOwnerReferences() // TODO

	// container probe
	probe := &coreapplyv1.ProbeApplyConfiguration{}
	grpc := &coreapplyv1.GRPCActionApplyConfiguration{}
	grpc.WithPort(fnrunv1alpha1.FnGRPCServerPort)
	grpc.WithService(podName)
	probe.WithGRPC(grpc)
	// container
	de := r.cache.GetDigestAndEntrypoint(image)
	if de == nil {
		return nil, errors.New("cannot return pod since digest is not initialized")
	}
	cmd := de.GetEntrypoint()

	container := &coreapplyv1.ContainerApplyConfiguration{}
	container.WithName(fnrunv1alpha1.FnContainerName)
	container.WithImage(r.image.Name)
	container.WithCommand(cmd...)
	container.WithReadinessProbe(probe)

	podSpec := &coreapplyv1.PodSpecApplyConfiguration{}
	podSpec.WithContainers(container)

	pod.WithSpec(podSpec)
	return pod, nil
}
*/
/*
func (r *controller) buildSvcPod(image fnrunv1alpha1.Image, podName string) (*corev1.Pod, error) {
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
								Port:    fnrunv1alpha1.FnGRPCServerPort,
								Service: pointer.String(podName),
							},
						},
					},
				},
			},
		},
	}, nil
}
*/
