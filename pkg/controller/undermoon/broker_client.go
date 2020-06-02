package undermoon

import (
	"fmt"
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
)

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

	resPayload := res.Result().(*brokerConfig)
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
