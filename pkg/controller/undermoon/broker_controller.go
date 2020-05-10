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

const brokerPort = 7799

type memBrokerController struct {
	r *ReconcileUndermoon
}

func newBrokerController(r *ReconcileUndermoon) *memBrokerController {
	return &memBrokerController{r: r}
}

func (con *memBrokerController) reconcileBroker(reqLogger logr.Logger, cr *undermoonv1alpha1.Undermoon) (*appsv1.StatefulSet, error) {
	if _, err := con.getOrCreateBrokerService(reqLogger, cr); err != nil {
		reqLogger.Error(err, "failed to create broker service", "Name", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
		return nil, err
	}

	brokerStatefulSet, err := con.getOrCreateBrokerStatefulSet(reqLogger, cr)
	if err != nil {
		reqLogger.Error(err, "failed to create etcd statefulset", "Name", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
		return nil, err
	}

	return brokerStatefulSet, nil
}

func (con *memBrokerController) getOrCreateBrokerService(reqLogger logr.Logger, cr *undermoonv1alpha1.Undermoon) (*corev1.Service, error) {
	service := createBrokerService(cr)

	if err := controllerutil.SetControllerReference(cr, service, con.r.scheme); err != nil {
		return nil, err
	}

	found := &corev1.Service{}
	err := con.r.client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new broker service", "Namespace", service.Namespace, "Name", service.Name)
		err = con.r.client.Create(context.TODO(), service)
		if err != nil {
			reqLogger.Error(err, "failed to create broker service")
			return nil, err
		}

		reqLogger.Info("Successfully created a new broker service", "Namespace", service.Namespace, "Name", service.Name)
		return service, nil
	} else if err != nil {
		reqLogger.Error(err, "failed to get broker service")
		return nil, err
	}

	reqLogger.Info("Skip reconcile: broker service already exists", "Namespace", found.Namespace, "Name", found.Name)
	return found, nil
}

func (con *memBrokerController) getOrCreateBrokerStatefulSet(reqLogger logr.Logger, cr *undermoonv1alpha1.Undermoon) (*appsv1.StatefulSet, error) {
	broker := createBrokerStatefulSet(cr)

	if err := controllerutil.SetControllerReference(cr, broker, con.r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed")
		return nil, err
	}

	// Check if this broker Statefulset already exists
	found := &appsv1.StatefulSet{}
	err := con.r.client.Get(context.TODO(), types.NamespacedName{Name: broker.Name, Namespace: broker.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new broker statefulset", "Namespace", broker.Namespace, "Name", broker.Name)
		err = con.r.client.Create(context.TODO(), broker)
		if err != nil {
			reqLogger.Error(err, "failed to create broker statefulset")
			return nil, err
		}

		// Statefulset created successfully - don't requeue
		return broker, nil
	} else if err != nil {
		reqLogger.Error(err, "failed to get broker statefulset")
		return nil, err
	}

	// broker already exists - don't requeue
	reqLogger.Info("Skip reconcile: broker statefulset already exists", "Namespace", found.Namespace, "Name", found.Name)
	return found, nil
}

func (con *memBrokerController) brokerReady(brokerStatefulSet *appsv1.StatefulSet) bool {
	return brokerStatefulSet.Status.ReadyReplicas == brokerNum
}