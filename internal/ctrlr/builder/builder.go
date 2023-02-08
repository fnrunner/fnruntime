package builder

import (
	"fmt"
	"strings"

	"github.com/fnrunner/fnruntime/internal/ctrlr/controller"
	"github.com/fnrunner/fnruntime/internal/ctrlr/controllers/eventhandler"
	"github.com/fnrunner/fnruntime/pkg/ctrlr/manager"
	"github.com/fnrunner/fnsyntax/pkg/ccsyntax"
	"github.com/fnrunner/fnutils/pkg/meta"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var newController = controller.New

type Builder interface {
	Build(r reconcile.Reconciler) (controller.Controller, error)
}

type builder struct {
	mgr   manager.Manager
	ceCtx ccsyntax.ConfigExecutionContext
	ge    chan event.GenericEvent

	globalPredicates []predicate.Predicate
	ctrl             controller.Controller
	ctrlOptions      controller.Options
}

type Config struct {
	Mgr          manager.Manager
	CeCtx        ccsyntax.ConfigExecutionContext
	GenericEvent chan event.GenericEvent
}

func New(c *Config, opts controller.Options) Builder {
	b := &builder{
		mgr:         c.Mgr,
		ceCtx:       c.CeCtx,
		ge:          c.GenericEvent,
		ctrlOptions: opts,
	}
	return b
}

func (blder *builder) Build(r reconcile.Reconciler) (controller.Controller, error) {
	if blder.mgr == nil {
		return nil, fmt.Errorf("must provide a non-nil Manager")
	}
	//if len(blder.ceCtx.GetFOW(ccsyntax.FOWFor)) != 1 {
	//	return nil, fmt.Errorf("cannot have more than 1 for")
	//}
	// Set the ControllerManagedBy
	if err := blder.doController(r); err != nil {
		return nil, err
	}

	// Set the Watch
	if err := blder.doWatch(); err != nil {
		return nil, err
	}
	return blder.ctrl, nil
}

func (blder *builder) doWatch() error {
	// handle For
	// we validate that there is only 1 for so we are ok here
	gvk := blder.ceCtx.GetForGVK()
	typeForSrc := meta.GetUnstructuredFromGVK(gvk)
	src := &source.Kind{Type: typeForSrc}
	hdler := &handler.EnqueueRequestForObject{}
	allPredicates := append(blder.globalPredicates, []predicate.Predicate{}...)
	if err := blder.ctrl.Watch(src, hdler, allPredicates...); err != nil {
		return err
	}
	// add the generic event watch to the for object
	if err := blder.ctrl.Watch(&source.Channel{Source: blder.ge}, hdler, allPredicates...); err != nil {
		return err
	}

	// handle Own
	// Watches the managed types
	for gvk := range blder.ceCtx.GetFOW(ccsyntax.FOWOwn) {
		obj := meta.GetUnstructuredFromGVK(&gvk)

		src := &source.Kind{Type: obj}
		hdler := &handler.EnqueueRequestForOwner{
			OwnerType:    typeForSrc,
			IsController: true,
		}
		allPredicates := append([]predicate.Predicate(nil), blder.globalPredicates...)
		allPredicates = append(allPredicates, []predicate.Predicate{}...)
		if err := blder.ctrl.Watch(src, hdler, allPredicates...); err != nil {
			return err
		}
	}

	// handle Watch
	for gvk, od := range blder.ceCtx.GetFOW(ccsyntax.FOWWatch) {
		//var obj client.Object
		obj := meta.GetUnstructuredFromGVK(&gvk)

		allPredicates := append([]predicate.Predicate(nil), blder.globalPredicates...)
		allPredicates = append(allPredicates, []predicate.Predicate{}...)

		// If the source of this watch is of type *source.Kind, project it.
		src := &source.Kind{Type: obj}

		eh := eventhandler.New(&eventhandler.Config{
			Client:         blder.mgr.GetClient(),
			RootVertexName: od[ccsyntax.OperationApply].RootVertexName,
			GVK:            &gvk,
			DAG:            od[ccsyntax.OperationApply].DAG,
		})

		if err := blder.ctrl.Watch(src, eh, allPredicates...); err != nil {
			return err
		}
	}

	return nil
}

func (blder *builder) getControllerName(gvk *schema.GroupVersionKind) string {
	if blder.ceCtx.GetName() != "" {
		return blder.ceCtx.GetName()
	}
	return strings.ToLower(gvk.Kind)
}

func (blder *builder) doController(r reconcile.Reconciler) error {
	globalOpts := blder.mgr.GetControllerOptions()

	ctrlOptions := blder.ctrlOptions
	if ctrlOptions.Reconciler == nil {
		ctrlOptions.Reconciler = r
	}

	gvk := blder.ceCtx.GetForGVK()
	// Setup concurrency.
	/*
		if ctrlOptions.MaxConcurrentReconciles == 0 {
			groupKind := gvk.GroupKind().String()

			if concurrency, ok := globalOpts.GroupKindConcurrency[groupKind]; ok && concurrency > 0 {
				ctrlOptions.MaxConcurrentReconciles = concurrency
			}
		}
	*/

	// Setup cache sync timeout.
	if ctrlOptions.CacheSyncTimeout == 0 && globalOpts.CacheSyncTimeout != nil {
		ctrlOptions.CacheSyncTimeout = *globalOpts.CacheSyncTimeout
	}

	controllerName := blder.getControllerName(gvk)

	// Setup the logger.
	if ctrlOptions.LogConstructor == nil {
		log := blder.mgr.GetLogger().WithValues(
			"controller", controllerName,
			"controllerGroup", gvk.Group,
			"controllerKind", gvk.Kind,
		)

		ctrlOptions.LogConstructor = func(req *reconcile.Request) logr.Logger {
			log := log
			if req != nil {
				log = log.WithValues(
					gvk.Kind, klog.KRef(req.Namespace, req.Name),
					"namespace", req.Namespace, "name", req.Name,
				)
			}
			return log
		}
	}
	// Build the controller and return.
	var err error
	blder.ctrl, err = newController(controllerName, blder.mgr, ctrlOptions)
	return err
}
