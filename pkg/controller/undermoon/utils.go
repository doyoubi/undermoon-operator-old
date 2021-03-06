package undermoon

import (
	"context"
	"sync"

	"github.com/go-redis/redis/v8"
	pkgerrors "github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const chunkNodeNumber int = 4
const halfChunkNodeNumber int = 2

var errRetryReconciliation = pkgerrors.New("retry reconciliation")

func createServiceGuard(createFunc func() (*corev1.Service, error)) (*corev1.Service, error) {
	var svc *corev1.Service
	var err error
	for i := 0; i != 3; i++ {
		svc, err = createFunc()
		if err == nil {
			return svc, err
		}
		if errors.IsAlreadyExists(err) {
			continue
		}
		return nil, err
	}
	return nil, err
}

func createStatefulSetGuard(createFunc func() (*appsv1.StatefulSet, error)) (*appsv1.StatefulSet, error) {
	var ss *appsv1.StatefulSet
	var err error
	for i := 0; i != 3; i++ {
		ss, err = createFunc()
		if err == nil {
			return ss, err
		}
		if errors.IsAlreadyExists(err) {
			continue
		}
		return nil, err
	}
	return nil, err
}

func getEndpoints(client client.Client, serviceName, namespace string) ([]corev1.EndpointAddress, error) {
	endpoints := &corev1.Endpoints{}
	// The endpoints names are the same as serviceName
	err := client.Get(context.TODO(), types.NamespacedName{Name: serviceName, Namespace: namespace}, endpoints)

	if err != nil && errors.IsNotFound(err) {
		return []corev1.EndpointAddress{}, nil
	} else if err != nil {
		return nil, err
	}

	addresses := []corev1.EndpointAddress{}
	for _, subnet := range endpoints.Subsets {
		addresses = append(addresses, subnet.Addresses...)
	}

	return addresses, nil
}

const podIPEnvStr = "$(UM_POD_IP)"
const podNameStr = "$(UM_POD_NAME)"

func podIPEnv() corev1.EnvVar {
	return corev1.EnvVar{
		Name: "UM_POD_IP",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "status.podIP",
			},
		},
	}
}

func podNameEnv() corev1.EnvVar {
	return corev1.EnvVar{
		Name: "UM_POD_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	}
}

type redisClientPool struct {
	lock    sync.Mutex
	clients map[string]*redis.Client
}

func newRedisClientPool() *redisClientPool {
	return &redisClientPool{
		lock:    sync.Mutex{},
		clients: make(map[string]*redis.Client),
	}
}

func (pool *redisClientPool) getClient(redisAddress string) *redis.Client {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	if client, ok := pool.clients[redisAddress]; ok {
		return client
	}

	client := redis.NewClient(&redis.Options{
		Addr: redisAddress,
	})
	pool.clients[redisAddress] = client
	return client
}

func genAntiAffinity(labels map[string]string, namespace, topologyKey string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 2,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						Namespaces:  []string{namespace},
						TopologyKey: topologyKey,
					},
				},
			},
		},
	}
}

func genPreStopHookLifeCycle(cmd []string) *corev1.Lifecycle {
	return &corev1.Lifecycle{
		PreStop: &corev1.Handler{
			Exec: &corev1.ExecAction{Command: cmd},
		},
	}
}
