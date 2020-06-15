package undermoon

import (
	"context"
	"time"

	undermoonv1alpha1 "github.com/doyoubi/undermoon-operator/pkg/apis/undermoon/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_undermoon")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Undermoon Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &ReconcileUndermoon{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
	r.brokerCon = newBrokerController(r)
	r.coodinatorCon = newCoordinatorController(r)
	r.storageCon = newStorageController(r)
	r.metaCon = newMetaController()
	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("undermoon-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Undermoon
	err = c.Watch(&source.Kind{Type: &undermoonv1alpha1.Undermoon{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Undermoon
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &undermoonv1alpha1.Undermoon{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileUndermoon implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileUndermoon{}

// ReconcileUndermoon reconciles a Undermoon object
type ReconcileUndermoon struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client        client.Client
	scheme        *runtime.Scheme
	brokerCon     *memBrokerController
	coodinatorCon *coordinatorController
	storageCon    *storageController
	metaCon       *metaController
}

// Reconcile reads that state of the cluster for a Undermoon object and makes changes based on the state read
// and what is in the Undermoon.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileUndermoon) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Undermoon")

	// Fetch the Undermoon instance
	instance := &undermoonv1alpha1.Undermoon{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	resource, err := r.createResources(reqLogger, instance)
	if err != nil {
		if err == errRetryReconciliation {
			return reconcile.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
		}
		return reconcile.Result{}, err
	}

	ready, err := r.brokerAndCoordinatorReady(resource, reqLogger, instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !ready {
		return reconcile.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
	}

	masterBrokerAddress, replicaAddresses, err := r.brokerCon.reconcileMaster(reqLogger, instance, resource.brokerService)
	if err != nil {
		if err == errRetryReconciliation {
			return reconcile.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
		}
		return reconcile.Result{}, err
	}

	err = r.coodinatorCon.configSetBroker(reqLogger, instance, resource.coordinatorService, masterBrokerAddress)
	if err != nil {
		return reconcile.Result{}, err
	}

	maxEpochFromServerProxy, err := r.storageCon.getMaxEpoch(reqLogger, resource.storageService, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = r.metaCon.fixBrokerEpoch(reqLogger, masterBrokerAddress, maxEpochFromServerProxy, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	proxies, err := r.storageCon.getServerProxies(reqLogger, resource.storageService, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	storageAllReady, err := r.storageCon.storageAllReady(resource.storageService, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	info, err := r.metaCon.reconcileMeta(reqLogger, masterBrokerAddress, replicaAddresses, proxies, instance, storageAllReady)
	if err != nil {
		if err == errRetryReconciliation {
			return reconcile.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
		}
		return reconcile.Result{}, err
	}

	err = r.metaCon.changeMeta(reqLogger, masterBrokerAddress, instance, info)
	if err != nil {
		if err == errRetryReconciliation {
			return reconcile.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
		}
		return reconcile.Result{}, err
	}

	// Ignore the proxies fetched from service.
	proxies = []serverProxyMeta{}
	info, err = r.metaCon.reconcileMeta(reqLogger, masterBrokerAddress, replicaAddresses, proxies, instance, storageAllReady)
	if err != nil {
		if err == errRetryReconciliation {
			return reconcile.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
		}
		return reconcile.Result{}, err
	}

	resource.storageStatefulSet, err = r.storageCon.scaleDownStorageStatefulSet(reqLogger, instance, resource.storageStatefulSet, info)
	if err != nil {
		if err == errRetryReconciliation {
			return reconcile.Result{Requeue: true, RequeueAfter: 3 * time.Second}, nil
		}
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

type umResource struct {
	brokerStatefulSet      *appsv1.StatefulSet
	coordinatorStatefulSet *appsv1.StatefulSet
	storageStatefulSet     *appsv1.StatefulSet
	brokerService          *corev1.Service
	coordinatorService     *corev1.Service
	storageService         *corev1.Service
}

func (r *ReconcileUndermoon) createResources(reqLogger logr.Logger, instance *undermoonv1alpha1.Undermoon) (*umResource, error) {
	brokerStatefulSet, brokerService, err := r.brokerCon.createBroker(reqLogger, instance)
	if err != nil {
		reqLogger.Error(err, "failed to create broker", "Name", instance.ObjectMeta.Name, "ClusterName", instance.Spec.ClusterName)
		return nil, err
	}

	coordinatorStatefulSet, coordinatorService, err := r.coodinatorCon.createCoordinator(reqLogger, instance)
	if err != nil {
		reqLogger.Error(err, "failed to create coordinator", "Name", instance.ObjectMeta.Name, "ClusterName", instance.Spec.ClusterName)
		return nil, err
	}

	storageStatefulSet, storageService, err := r.storageCon.createStorage(reqLogger, instance)
	if err != nil {
		reqLogger.Error(err, "failed to create storage", "Name", instance.ObjectMeta.Name, "ClusterName", instance.Spec.ClusterName)
		return nil, err
	}

	return &umResource{
		brokerStatefulSet:      brokerStatefulSet,
		coordinatorStatefulSet: coordinatorStatefulSet,
		storageStatefulSet:     storageStatefulSet,
		brokerService:          brokerService,
		coordinatorService:     coordinatorService,
		storageService:         storageService,
	}, nil
}

func (r *ReconcileUndermoon) brokerAndCoordinatorReady(resource *umResource, reqLogger logr.Logger, instance *undermoonv1alpha1.Undermoon) (bool, error) {
	ready, err := r.brokerCon.brokerReady(resource.brokerStatefulSet, resource.brokerService)
	if err != nil {
		reqLogger.Error(err, "failed to check broker ready", "Name", instance.ObjectMeta.Name, "ClusterName", instance.Spec.ClusterName)
		return false, err
	}
	if !ready {
		reqLogger.Info("broker statefulset not ready", "Name", instance.ObjectMeta.Name, "ClusterName", instance.Spec.ClusterName)
		return false, nil
	}

	ready, err = r.coodinatorCon.coordinatorReady(resource.coordinatorStatefulSet, resource.coordinatorService)
	if err != nil {
		reqLogger.Error(err, "failed to check coordinator ready", "Name", instance.ObjectMeta.Name, "ClusterName", instance.Spec.ClusterName)
		return false, err
	}
	if !ready {
		reqLogger.Info("coordinator statefulset not ready", "Name", instance.ObjectMeta.Name, "ClusterName", instance.Spec.ClusterName)
		return false, nil
	}

	return true, nil
}
