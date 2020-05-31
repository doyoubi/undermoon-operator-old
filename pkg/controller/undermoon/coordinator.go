package undermoon

import (
	"fmt"

	undermoonv1alpha1 "github.com/doyoubi/undermoon-operator/pkg/apis/undermoon/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const coordinatorPort = 6699
const coordinatorNum int32 = 3
const coordinatorContainerName = "coordinator"
const undermoonServiceTypeCoordinator = "coordinator"

func createCoordinatorService(cr *undermoonv1alpha1.Undermoon) *corev1.Service {
	undermoonName := cr.ObjectMeta.Name

	labels := map[string]string{
		"undermoonService":     undermoonServiceTypeCoordinator,
		"undermoonName":        undermoonName,
		"undermoonClusterName": cr.Spec.ClusterName,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CoordinatorServiceName(undermoonName),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "coordinator-port",
					Port:     coordinatorPort,
					Protocol: corev1.ProtocolTCP,
				},
			},
			ClusterIP: "None", // Make it a headless service
			Selector:  labels,
		},
	}
}

// CoordinatorServiceName defines the service for coordinator statefulsets.
func CoordinatorServiceName(clusterName string) string {
	return fmt.Sprintf("%s-coordinator-svc", clusterName)
}

func createCoordinatorStatefulSet(cr *undermoonv1alpha1.Undermoon) *appsv1.StatefulSet {
	labels := map[string]string{
		"undermoonService":     "coordinator",
		"undermoonName":        cr.ObjectMeta.Name,
		"undermoonClusterName": cr.Spec.ClusterName,
	}

	env := []corev1.EnvVar{
		{
			Name:  "RUST_LOG",
			Value: "undermoon=info,coordinator=info",
		},
		{
			Name:  "UNDERMOON_ADDRESS",
			Value: "0.0.0.0:6699",
		},
		{
			Name:  "UNDERMOON_BROKER_ADDRESS",
			Value: "",
		},
		{
			Name: "UNDERMOON_REPORTER_ID",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "UNDERMOON_THREAD_NUMBER",
			Value: "2",
		},
	}
	container := corev1.Container{
		Name:            coordinatorContainerName,
		Image:           cr.Spec.UndermoonImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"coordinator"},
		Env:             env,
	}
	podSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container},
		},
	}

	replicaNum := coordinatorNum

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      CoordinatorStatefulSetName(cr.ObjectMeta.Name),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector:            &metav1.LabelSelector{MatchLabels: labels},
			ServiceName:         CoordinatorServiceName(cr.ObjectMeta.Name),
			Replicas:            &replicaNum,
			Template:            podSpec,
			PodManagementPolicy: appsv1.ParallelPodManagement,
		},
	}
}

// CoordinatorStatefulSetName defines the statefulset for memory coordinator.
func CoordinatorStatefulSetName(undermoonName string) string {
	return fmt.Sprintf("%s-coordinator-ss", undermoonName)
}
