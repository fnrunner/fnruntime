package imagecontroller

import (
	"context"

	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	coreapplyv1 "k8s.io/client-go/applyconfigurations/core/v1"
)

func (r *controller) applyService(ctx context.Context, podName string) (*corev1.Service, error) {
	// apply service
	return r.client.CoreV1().Services(r.namespace).Apply(ctx, r.buildService(r.image, podName), metav1.ApplyOptions{})
}

func (r *controller) deleteService(ctx context.Context, podName string) error {
	// apply service
	return r.client.CoreV1().Services(r.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

func (r *controller) buildService(image fnrunv1alpha1.Image, podName string) *coreapplyv1.ServiceApplyConfiguration {
	svc := &coreapplyv1.ServiceApplyConfiguration{}
	svc.WithAPIVersion("v1")
	svc.WithKind("Pod")
	svc.WithNamespace(r.namespace)
	svc.WithName(podName)
	svc.WithLabels(map[string]string{
		fnrunv1alpha1.FunctionLabelKey: podName,
	})
	svc.WithOwnerReferences() // TODO

	// spec
	svcSpec := &coreapplyv1.ServiceSpecApplyConfiguration{}
	svcSpec.WithSelector(map[string]string{fnrunv1alpha1.FunctionLabelKey: podName})
	svcSpec.WithClusterIP("None")

	svcPort := &coreapplyv1.ServicePortApplyConfiguration{}
	svcPort.WithName("grpc")
	svcPort.WithPort(fnrunv1alpha1.FnGRPCServerPort)
	svcPort.WithTargetPort(intstr.FromInt(fnrunv1alpha1.FnGRPCServerPort))
	svcPort.WithProtocol(corev1.Protocol("TCP"))

	svcSpec.WithPorts(svcPort)
	svc.WithSpec(svcSpec)

	return svc
}
