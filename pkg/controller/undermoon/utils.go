package undermoon

import (
	"context"
	"sync"

	"github.com/go-redis/redis/v8"
	pkgerrors "github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const chunkNodeNumber int = 4

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
