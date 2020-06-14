package undermoon

import (
	"fmt"

	undermoonv1alpha1 "github.com/doyoubi/undermoon-operator/pkg/apis/undermoon/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const serverProxyPort = 5299
const redisPort1 = 7001
const redisPort2 = 7002
const serverProxyContainerName = "server-proxy"
const redisContainerName = "redis"
const undermoonServiceTypeStorage = "storage"

func createStorageService(cr *undermoonv1alpha1.Undermoon) *corev1.Service {
	undermoonName := cr.ObjectMeta.Name

	labels := map[string]string{
		"undermoonService":     undermoonServiceTypeStorage,
		"undermoonName":        undermoonName,
		"undermoonClusterName": cr.Spec.ClusterName,
	}

	// This service is only used to query the hosts and ips of the server proxies.
	// It will not be used directly.
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StorageServiceName(undermoonName),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "server-proxy-port",
					Port:     serverProxyPort,
					Protocol: corev1.ProtocolTCP,
				},
			},
			ClusterIP: "None", // Make it a headless service
			Selector:  labels,
		},
	}
}

// StorageServiceName defines the service for storage StatefulSet.
func StorageServiceName(clusterName string) string {
	return fmt.Sprintf("%s-storage-svc", clusterName)
}

func createStorageStatefulSet(cr *undermoonv1alpha1.Undermoon) *appsv1.StatefulSet {
	labels := map[string]string{
		"undermoonService":     undermoonServiceTypeStorage,
		"undermoonName":        cr.ObjectMeta.Name,
		"undermoonClusterName": cr.Spec.ClusterName,
	}

	env := []corev1.EnvVar{
		podIPEnv(),
		{
			Name:  "RUST_LOG",
			Value: "undermoon=info,server_proxy=info",
		},
		{
			Name:  "UNDERMOON_ADDRESS",
			Value: fmt.Sprintf("0.0.0.0:%d", serverProxyPort),
		},
		// UNDERMOON_ANNOUNCE_ADDRESS is set in the command
		{
			Name:  "UNDERMOON_AUTO_SELECT_CLUSTER",
			Value: "true",
		},
		{
			Name:  "UNDERMOON_SLOWLOG_LEN",
			Value: "1024",
		},
		{
			Name:  "UNDERMOON_SLOWLOG_LOG_SLOWER_THAN",
			Value: "10000",
		},
		{
			Name:  "UNDERMOON_SLOWLOG_SAMPLE_RATE",
			Value: "1000",
		},
		{
			Name:  "UNDERMOON_SESSION_CHANNEL_SIZE",
			Value: "4096",
		},
		{
			Name:  "UNDERMOON_BACKEND_CHANNEL_SIZE",
			Value: "4096",
		},
		{
			Name:  "UNDERMOON_BACKEND_BATCH_MIN_TIME",
			Value: "20000",
		},
		{
			Name:  "UNDERMOON_BACKEND_BATCH_MAX_TIME",
			Value: "400000",
		},
		{
			Name:  "UNDERMOON_SESSION_BATCH_MIN_TIME",
			Value: "20000",
		},
		{
			Name:  "UNDERMOON_SESSION_BATCH_MAX_TIME",
			Value: "400000",
		},
		{
			Name:  "UNDERMOON_ACTIVE_REDIRECTION",
			Value: "false",
		},
	}
	serverProxyContainer := corev1.Container{
		Name:            serverProxyContainerName,
		Image:           cr.Spec.UndermoonImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"sh",
			"-c",
			fmt.Sprintf("UNDERMOON_ANNOUNCE_ADDRESS=\"%s:%d\" server_proxy", podIPEnvStr, serverProxyPort),
		},
		Env: env,
	}
	redisContainer1 := genRedisContainer(1, cr.Spec.RedisImage, cr.Spec.MaxMemory, redisPort1)
	redisContainer2 := genRedisContainer(2, cr.Spec.RedisImage, cr.Spec.MaxMemory, redisPort2)
	podSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				serverProxyContainer,
				redisContainer1,
				redisContainer2,
			},
		},
	}

	replicaNum := int32(cr.Spec.ChunkNumber) * 2

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      StorageStatefulSetName(cr.ObjectMeta.Name),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector:            &metav1.LabelSelector{MatchLabels: labels},
			ServiceName:         StorageServiceName(cr.ObjectMeta.Name),
			Replicas:            &replicaNum,
			Template:            podSpec,
			PodManagementPolicy: appsv1.ParallelPodManagement,
		},
	}
}

func genRedisContainer(index uint32, redisImage string, maxMemory, port uint32) corev1.Container {
	portStr := fmt.Sprintf("%d", port)
	return corev1.Container{
		Name:            fmt.Sprintf("%s-%d", redisContainerName, index),
		Image:           redisImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"redis-server"},
		Args: []string{
			"--maxmemory",
			fmt.Sprintf("%dMB", maxMemory),
			"--port",
			portStr,
			"--slave-announce-port",
			portStr,
			"--slave-announce-ip",
			podIPEnvStr,
			"--maxmemory-policy",
			"allkeys-lru",
		},
		Env: []corev1.EnvVar{podIPEnv()},
	}
}

// StorageStatefulSetName defines the StatefulSet for server proxy.
func StorageStatefulSetName(undermoonName string) string {
	return fmt.Sprintf("%s-storage-ss", undermoonName)
}

func genStorageFQDN(storageName, clusterName, namespace string) string {
	// pod-specific-string.serviceName.default.svc.cluster.local
	return fmt.Sprintf("%s.%s.%s.svc.cluster.local", storageName, StorageServiceName(clusterName), namespace)
}

func genStorageFQDNFromName(name string, cr *undermoonv1alpha1.Undermoon) string {
	host := genStorageFQDN(name, cr.ObjectMeta.Name, cr.ObjectMeta.Namespace)
	return host
}

func genStorageAddressFromName(name string, cr *undermoonv1alpha1.Undermoon) string {
	host := genStorageFQDNFromName(name, cr)
	addr := fmt.Sprintf("%s:%d", host, serverProxyPort)
	return addr
}
