package undermoon

import (
	"context"
	"sync"

	"github.com/go-redis/redis/v8"
)

type coordinatorClient struct {
	redisClient *redis.Client
}

func newCoordinatorClient(address string) *coordinatorClient {
	return &coordinatorClient{
		redisClient: redis.NewClient(&redis.Options{
			Addr: address,
		}),
	}
}

func (client *coordinatorClient) setBrokerAddress(brokerAddress string) error {
	cmd := redis.NewStringCmd(context.TODO(), "CONFIG", "SET", "brokers", brokerAddress)
	err := client.redisClient.Process(context.TODO(), cmd)
	if err != nil {
		return err
	}
	_, err = cmd.Result()
	return err
}

type coordinatorClientPool struct {
	lock    sync.Mutex
	clients map[string]*coordinatorClient
}

func newCoordinatorClientPool() *coordinatorClientPool {
	return &coordinatorClientPool{
		lock:    sync.Mutex{},
		clients: make(map[string]*coordinatorClient),
	}
}

func (pool *coordinatorClientPool) getClient(coordAddress string) *coordinatorClient {
	pool.lock.Lock()
	defer pool.lock.Unlock()

	if client, ok := pool.clients[coordAddress]; ok {
		return client
	}

	client := newCoordinatorClient(coordAddress)
	pool.clients[coordAddress] = client
	return client
}

func (pool *coordinatorClientPool) setBrokerAddress(coordAddress, brokerAddress string) error {
	c := pool.getClient(coordAddress)
	return c.setBrokerAddress(brokerAddress)
}
