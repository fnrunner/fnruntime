package fnreconciler

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

/*
type Result struct {
	// Done tells the Controller to requeue the reconcile key.  Defaults to false.
	Done bool

	// RequeueAfter if greater than 0, tells the Controller to requeue the reconcile key after the Duration.
	// Implies that Requeue is true, there is no need to set Requeue to true at the same time as RequeueAfter.
	RequeueAfter time.Duration
}
*/

type Reconciler interface {
	Reconcile(ctx context.Context, key types.NamespacedName) (bool, error)
}

func NewNopReconciler() Reconciler {
	return &nopReconciler{}
}

type nopReconciler struct{}

func (r *nopReconciler) Reconcile(ctx context.Context, key types.NamespacedName) (bool, error) {
	return false, nil
}
