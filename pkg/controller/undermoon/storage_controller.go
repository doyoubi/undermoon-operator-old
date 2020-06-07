package undermoon

import (
	"context"

	undermoonv1alpha1 "github.com/doyoubi/undermoon-operator/pkg/apis/undermoon/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type storageController struct {
	r *ReconcileUndermoon
}

func newStorageController(r *ReconcileUndermoon) *storageController {
	return &storageController{r: r}
}

func (con *storageController) createStorage(reqLogger logr.Logger, cr *undermoonv1alpha1.Undermoon) (*appsv1.Deployment, *corev1.Service, error) {
	storageService, err := createServiceGuard(func() (*corev1.Service, error) {
		return con.getOrCreateStorageService(reqLogger, cr)
	})
	if err != nil {
		reqLogger.Error(err, "failed to create storage service", "Name", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
		return nil, nil, err
	}

	storageDeployment, err := createDeploymentGuard(func() (*appsv1.Deployment, error) {
		return con.getOrCreateStorageDeployment(reqLogger, cr)
	})
	if err != nil {
		reqLogger.Error(err, "failed to create storage deployment", "Name", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
		return nil, nil, err
	}

	// Only update replica number here for scaling out.
	if int32(cr.Spec.ChunkNumber)*2 > *storageDeployment.Spec.Replicas {
		storageDeployment, err = con.updateStorageDeployment(reqLogger, cr, storageDeployment)
		if err != nil {
			if errors.IsConflict(err) {
				reqLogger.Info("Conflict on updating storage deployment. Try again.", "error", err)
				return nil, nil, errRetryReconciliation
			}
			reqLogger.Error(err, "failed to update storage deployment", "Name", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
			return nil, nil, err
		}
	}

	return storageDeployment, storageService, nil
}

func (con *storageController) getOrCreateStorageService(reqLogger logr.Logger, cr *undermoonv1alpha1.Undermoon) (*corev1.Service, error) {
	service := createStorageService(cr)

	if err := controllerutil.SetControllerReference(cr, service, con.r.scheme); err != nil {
		return nil, err
	}

	found := &corev1.Service{}
	err := con.r.client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new storage service", "Namespace", service.Namespace, "Name", service.Name)
		err = con.r.client.Create(context.TODO(), service)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				reqLogger.Info("storage service already exists")
			} else {
				reqLogger.Error(err, "failed to create storage service")
			}
			return nil, err
		}

		reqLogger.Info("Successfully created a new storage service", "Namespace", service.Namespace, "Name", service.Name)
		return service, nil
	} else if err != nil {
		reqLogger.Error(err, "failed to get storage service")
		return nil, err
	}

	reqLogger.Info("Skip reconcile: storage service already exists", "Namespace", found.Namespace, "Name", found.Name)
	return found, nil
}

func (con *storageController) getOrCreateStorageDeployment(reqLogger logr.Logger, cr *undermoonv1alpha1.Undermoon) (*appsv1.Deployment, error) {
	storage := createStorageDeployment(cr)

	if err := controllerutil.SetControllerReference(cr, storage, con.r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed")
		return nil, err
	}

	// Check if this storage deployment already exists
	found := &appsv1.Deployment{}
	err := con.r.client.Get(context.TODO(), types.NamespacedName{Name: storage.Name, Namespace: storage.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new storage deployment", "Namespace", storage.Namespace, "Name", storage.Name)
		err = con.r.client.Create(context.TODO(), storage)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				reqLogger.Info("storage deployment already exists")
			} else {
				reqLogger.Error(err, "failed to create storage deployment")
			}
			return nil, err
		}

		// deployment created successfully - don't requeue
		return storage, nil
	} else if err != nil {
		reqLogger.Error(err, "failed to get storage deployment", "Name", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
		return nil, err
	}

	// storage already exists - don't requeue
	reqLogger.Info("Skip reconcile: storage deployment already exists", "Namespace", found.Namespace, "Name", found.Name)
	return found, nil
}

func (con *storageController) updateStorageDeployment(reqLogger logr.Logger, cr *undermoonv1alpha1.Undermoon, storage *appsv1.Deployment) (*appsv1.Deployment, error) {
	replicaNum := int32(cr.Spec.ChunkNumber) * 2
	storage.Spec.Replicas = &replicaNum

	err := con.r.client.Update(context.TODO(), storage)
	if err != nil {
		reqLogger.Error(err, "failed to update storage deployment", "Name", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
		return nil, err
	}

	return storage, nil
}

func (con *storageController) getServiceEndpointsNum(storageService *corev1.Service) (int, error) {
	endpoints, err := getEndpoints(con.r.client, storageService.Name, storageService.Namespace)
	if err != nil {
		return 0, err
	}
	return len(endpoints), nil
}

func (con *storageController) storageReady(storageDeployment *appsv1.Deployment, storageService *corev1.Service, cr *undermoonv1alpha1.Undermoon) (bool, error) {
	n, err := con.getServiceEndpointsNum(storageService)
	if err != nil {
		return false, err
	}
	serverProxyNum := cr.Spec.ChunkNumber * 2
	ready := storageDeployment.Status.ReadyReplicas >= int32(serverProxyNum)-1 && n >= int(serverProxyNum-1)
	return ready, nil
}

func (con *storageController) storageAllReady(storageDeployment *appsv1.Deployment, storageService *corev1.Service, cr *undermoonv1alpha1.Undermoon) (bool, error) {
	n, err := con.getServiceEndpointsNum(storageService)
	if err != nil {
		return false, err
	}
	serverProxyNum := cr.Spec.ChunkNumber * 2
	ready := storageDeployment.Status.ReadyReplicas >= int32(serverProxyNum) && n >= int(serverProxyNum)
	return ready, err
}

func (con *storageController) getServerProxiesIPs(reqLogger logr.Logger, storageService *corev1.Service, cr *undermoonv1alpha1.Undermoon) ([]string, error) {
	endpoints, err := getEndpoints(con.r.client, storageService.Name, storageService.Namespace)
	if err != nil {
		reqLogger.Error(err, "Failed to get endpoints of server proxies", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
		return nil, err
	}

	ips := []string{}
	for _, endpoint := range endpoints {
		ips = append(ips, endpoint.IP)
	}

	return ips, nil
}
