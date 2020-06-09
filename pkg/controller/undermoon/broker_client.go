package undermoon

import (
	"fmt"
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
)

const errStrAlreadyExists = "ALREADY_EXISTED"
const errStrNodeNumAlreadyEnough = "NODE_NUM_ALREADY_ENOUGH"
const errStrMigrationRunning = "MIGRATION_RUNNING"
const errStrFreeNodeNotFound = "FREE_NODE_NOT_FOUND"
const errStrFreeNodeFound = "FREE_NODE_FOUND"
const errStrInvalidNodeNumber = "INVALID_NODE_NUMBER"
const errSlotsAlreadyEven = "SLOTS_ALREADY_EVEN"

var errMigrationRunning = errors.New("MIGRATION_RUNNING")

type errorResponse struct {
	Error string `json:"error"`
}

type brokerClient struct {
	httpClient *resty.Client
}

func newBrokerClient() *brokerClient {
	httpClient := resty.New()
	httpClient.SetHeader("Content-Type", "application/json")
	return &brokerClient{
		httpClient: httpClient,
	}
}

type brokerConfig struct {
	ReplicaAddresses []string `json:"replica_addresses"`
}

func (client *brokerClient) getReplicaAddresses(address string) ([]string, error) {
	url := fmt.Sprintf("http://%s/api/v2/config", address)
	payload := brokerConfig{}
	res, err := client.httpClient.R().SetResult(payload).Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != 200 {
		return nil, errors.Errorf("Failed to get replica addresses from broker: invalid status code %d", res.StatusCode())
	}

	resPayload, ok := res.Result().(*brokerConfig)
	if !ok {
		content := res.Body()
		return nil, errors.Errorf("Failed to get replica addresses from broker: invalid response payload %s", string(content))
	}

	addresses := resPayload.ReplicaAddresses
	return addresses, nil
}

func (client *brokerClient) storeReplicaAddresses(address string, replicaAddresses []string) error {
	payload := &brokerConfig{
		ReplicaAddresses: replicaAddresses,
	}
	url := fmt.Sprintf("http://%s/api/v2/config", address)
	res, err := client.httpClient.R().
		SetBody(payload).
		Put(url)
	if err != nil {
		return err
	}

	if res.StatusCode() != 200 {
		return errors.Errorf("Failed to store replica addresses to broker: invalid status code %d", res.StatusCode())
	}
	return nil
}

func (client *brokerClient) getEpoch(address string) (uint64, error) {
	url := fmt.Sprintf("http://%s/api/v2/epoch", address)
	res, err := client.httpClient.R().Get(url)
	if err != nil {
		return 0, err
	}

	if res.StatusCode() != 200 {
		return 0, errors.Errorf("Failed to get broker epoch: invalid status code %d", res.StatusCode())
	}

	body := res.Body()
	epoch, err := strconv.ParseUint(string(body), 10, 64)
	if err != nil {
		return 0, errors.Errorf("Invalid epoch from broker: %s", string(body))
	}

	return epoch, nil
}

type queryServerProxyResponse struct {
	Addresses []string `json:"addresses"`
}

func (client *brokerClient) getServerProxies(address string) ([]string, error) {
	url := fmt.Sprintf("http://%s/api/v2/proxies/addresses", address)
	res, err := client.httpClient.R().SetResult(&queryServerProxyResponse{}).Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode() != 200 {
		content := res.Body()
		return nil, errors.Errorf("Failed to register server proxy: invalid status code %d: %s", res.StatusCode(), string(content))
	}

	resultPayload := res.Result().(*queryServerProxyResponse)
	return resultPayload.Addresses, nil
}

type serverProxyMeta struct {
	ProxyAddress   string    `json:"proxy_address"`
	RedisAddresses [2]string `json:"nodes"`
	Host           string    `host:"host"`
}

func newServerProxyMeta(ip, nodeIP string) serverProxyMeta {
	return serverProxyMeta{
		ProxyAddress: fmt.Sprintf("%s:%d", ip, serverProxyPort),
		RedisAddresses: [2]string{
			fmt.Sprintf("%s:%d", ip, redisPort1),
			fmt.Sprintf("%s:%d", ip, redisPort2),
		},
		Host: nodeIP,
	}
}

func (client *brokerClient) registerServerProxy(address string, proxy serverProxyMeta) error {
	url := fmt.Sprintf("http://%s/api/v2/proxies/meta", address)
	res, err := client.httpClient.R().SetBody(&proxy).Post(url)
	if err != nil {
		return err
	}

	if res.StatusCode() != 200 && res.StatusCode() != 409 {
		content := res.Body()
		return errors.Errorf("Failed to register server proxy: invalid status code %d: %s", res.StatusCode(), string(content))
	}

	return nil
}

func (client *brokerClient) deregisterServerProxy(address string, proxyAddress string) error {
	url := fmt.Sprintf("http://%s/api/v2/proxies/meta", address)
	res, err := client.httpClient.R().Delete(url)
	if err != nil {
		return err
	}

	if res.StatusCode() != 200 && res.StatusCode() != 400 {
		content := res.Body()
		return errors.Errorf("Failed to register server proxy: invalid status code %d: %s", res.StatusCode(), string(content))
	}

	return nil
}

type createClusterPayload struct {
	NodeNumber int `json:"node_number"`
}

func (client *brokerClient) createCluster(address, clusterName string, chunkNumber int) error {
	url := fmt.Sprintf("http://%s/api/v2/clusters/meta/%s", address, clusterName)
	payload := &createClusterPayload{
		NodeNumber: chunkNumber * chunkNodeNumber,
	}
	res, err := client.httpClient.R().SetBody(payload).SetError(&errorResponse{}).Post(url)
	if err != nil {
		return err
	}

	if res.StatusCode() == 200 {
		return nil
	}

	if res.StatusCode() == 409 {
		response, ok := res.Error().(*errorResponse)
		if ok && response.Error == errStrAlreadyExists {
			return nil
		}
	}

	content := res.Body()
	return errors.Errorf("Failed to register server proxy: invalid status code %d: %s", res.StatusCode(), string(content))
}

type queryClusterNamesPayload struct {
	Names []string `json:"names"`
}

func (client *brokerClient) clusterExists(address, clusterName string) (bool, error) {
	url := fmt.Sprintf("http://%s/api/v2/clusters/names", address)
	res, err := client.httpClient.R().SetResult(&queryClusterNamesPayload{}).Get(url)
	if err != nil {
		return false, err
	}

	if res.StatusCode() != 200 {
		content := res.Body()
		return false, errors.Errorf("Failed to register server proxy: invalid status code %d: %s", res.StatusCode(), string(content))
	}

	response, ok := res.Result().(*queryClusterNamesPayload)
	if !ok {
		content := res.Body()
		return false, errors.Errorf("Failed to get cluster names: invalid response payload %s", string(content))
	}

	for _, name := range response.Names {
		if name == clusterName {
			return true, nil
		}
	}
	return false, nil
}

type addNodesPayload struct {
	ClusterNodeNumber int `json:"cluster_node_number"`
}

func (client *brokerClient) scaleNodes(address, clusterName string, chunkNumber int) error {
	nodeNumber := chunkNumber * chunkNodeNumber
	url := fmt.Sprintf("http://%s/api/v2/clusters/migrations/auto/%s/%d", address, clusterName, nodeNumber)
	res, err := client.httpClient.R().SetError(&errorResponse{}).Post(url)
	if err != nil {
		return err
	}

	if res.StatusCode() == 200 {
		return nil
	}

	if res.StatusCode() == 409 || res.StatusCode() == 400 {
		response, ok := res.Error().(*errorResponse)
		if ok && response.Error == errStrMigrationRunning {
			return errMigrationRunning
		}
	}

	content := res.Body()
	return errors.Errorf("Failed to expand slots: invalid status code %d: %s", res.StatusCode(), string(content))
}
