package undermoon

import (
	"fmt"

	undermoonv1alpha1 "github.com/doyoubi/undermoon-operator/pkg/apis/undermoon/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const brokerNum int32 = 3
const brokerContainerName = "broker"

func createBrokerService(cr *undermoonv1alpha1.Undermoon) *corev1.Service {
	clusterName := cr.ObjectMeta.Name

	labels := map[string]string{
		"undermoonService":     "broker",
		"undermoonClusterName": clusterName,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BrokerServiceName(clusterName),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "broker-port",
					Port:     brokerPort,
					Protocol: corev1.ProtocolTCP,
				},
			},
			ClusterIP: "None", // Make it a headless service
			Selector:  labels,
		},
	}
}

// BrokerServiceName defines the service for broker statefulsets.
func BrokerServiceName(clusterName string) string {
	return fmt.Sprintf("%s-broker-svc", clusterName)
}

func createBrokerStatefulSet(cr *undermoonv1alpha1.Undermoon) *appsv1.StatefulSet {
	labels := map[string]string{
		"undermoonService":     "broker",
		"undermoonClusterName": cr.ObjectMeta.Name,
	}

	env := []corev1.EnvVar{
		{
			Name:  "RUST_LOG",
			Value: "undermoon=info,mem_broker=info",
		},
		{
			Name:  "UNDERMOON_ADDRESS",
			Value: "0.0.0.0:7799",
		},
		{
			Name:  "UNDERMOON_FAILURE_TTL",
			Value: "60",
		},
		{
			Name:  "UNDERMOON_FAILURE_QUORUM",
			Value: "2",
		},
		{
			Name:  "UNDERMOON_MIGRATION_LIMIT",
			Value: "2",
		},
		{
			Name:  "UNDERMOON_RECOVER_FROM_META_FILE",
			Value: "true",
		},
		{
			Name:  "UNDERMOON_META_FILENAME",
			Value: "metadata",
		},
		{
			Name:  "UNDERMOON_AUTO_UPDATE_META_FILE",
			Value: "true",
		},
		{
			Name:  "UNDERMOON_UPDATE_META_FILE_INTERVAL",
			Value: "10",
		},
		{
			Name:  "UNDERMOON_REPLICA_ADDRESSES",
			Value: "",
		},
		{
			Name:  "UNDERMOON_SYNC_META_INTERVAL",
			Value: "5",
		},
		{
			Name:  "UNDERMOON_DEBUG",
			Value: "false",
		},
	}
	container := corev1.Container{
		Name:            brokerContainerName,
		Image:           cr.Spec.UndermoonImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"mem_broker"},
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

	replicaNum := brokerNum

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      BrokerStatefulSetName(cr.ObjectMeta.Name),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector:            &metav1.LabelSelector{MatchLabels: labels},
			ServiceName:         BrokerServiceName(cr.ObjectMeta.Name),
			Replicas:            &replicaNum,
			Template:            podSpec,
			PodManagementPolicy: appsv1.ParallelPodManagement,
		},
	}
}

// BrokerStatefulSetName defines the statefulset for memory broker.
func BrokerStatefulSetName(clusterName string) string {
	return fmt.Sprintf("%s-broker-ss", clusterName)
}