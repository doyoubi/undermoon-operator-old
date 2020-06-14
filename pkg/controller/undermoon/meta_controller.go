package undermoon

import (
	undermoonv1alpha1 "github.com/doyoubi/undermoon-operator/pkg/apis/undermoon/v1alpha1"
	"github.com/go-logr/logr"
)

type metaController struct {
	client *brokerClient
}

func newMetaController() *metaController {
	client := newBrokerClient()
	return &metaController{client: client}
}

func (con *metaController) reconcileMeta(reqLogger logr.Logger, masterBrokerAddress string, replicaAddresses []string, proxies []serverProxyMeta, cr *undermoonv1alpha1.Undermoon, storageAllReady bool) (*clusterInfo, error) {
	err := con.setBrokerReplicas(reqLogger, masterBrokerAddress, replicaAddresses, cr)
	if err != nil {
		return nil, err
	}

	err = con.reconcileServerProxyRegistry(reqLogger, masterBrokerAddress, proxies, cr)
	if err != nil {
		return nil, err
	}

	if !storageAllReady {
		return nil, errRetryReconciliation
	}

	err = con.createCluster(reqLogger, masterBrokerAddress, cr)
	if err != nil {
		return nil, err
	}

	info, err := con.getClusterInfo(reqLogger, masterBrokerAddress, cr)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (con *metaController) setBrokerReplicas(reqLogger logr.Logger, masterBrokerAddress string, replicaAddresses []string, cr *undermoonv1alpha1.Undermoon) error {
	err := con.client.setBrokerReplicas(masterBrokerAddress, replicaAddresses)
	if err != nil {
		reqLogger.Error(err, "failed to set broker replicas", "masterBrokerAddress", masterBrokerAddress, "Name", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
		return err
	}
	return nil
}

func (con *metaController) changeMeta(reqLogger logr.Logger, masterBrokerAddress string, cr *undermoonv1alpha1.Undermoon, info *clusterInfo) error {
	if info.IsMigrating {
		return errRetryReconciliation
	}

	err := con.changeNodeNumber(reqLogger, masterBrokerAddress, cr)
	if err != nil {
		if err == errMigrationRunning {
			return errRetryReconciliation
		}
		return err
	}

	return nil
}

func (con *metaController) reconcileServerProxyRegistry(reqLogger logr.Logger, masterBrokerAddress string, proxies []serverProxyMeta, cr *undermoonv1alpha1.Undermoon) error {
	err := con.registerServerProxies(reqLogger, masterBrokerAddress, proxies, cr)
	if err != nil {
		return err
	}

	err = con.deregisterServerProxies(reqLogger, masterBrokerAddress, proxies, cr)
	if err != nil {
		return err
	}

	return nil
}

func (con *metaController) registerServerProxies(reqLogger logr.Logger, masterBrokerAddress string, proxies []serverProxyMeta, cr *undermoonv1alpha1.Undermoon) error {
	for _, proxy := range proxies {
		err := con.client.registerServerProxy(masterBrokerAddress, proxy)
		if err != nil {
			reqLogger.Error(err, "failed to register server proxy", "proxy", proxy, "Name", cr.ObjectMeta.Name, "ClusterName", cr.Spec.ClusterName)
		}
	}
	return nil
}

func (con *metaController) deregisterServerProxies(reqLogger logr.Logger, masterBrokerAddress string, proxies []serverProxyMeta, cr *undermoonv1alpha1.Undermoon) error {
	existingProxies, err := con.client.getServerProxies(masterBrokerAddress)
	if err != nil {
		reqLogger.Error(err, "failed to get server proxy addresses",
			"Name", cr.ObjectMeta.Name,
			"ClusterName", cr.Spec.ClusterName)
		return err
	}

	keepSet := make(map[string]bool, 0)
	for _, proxy := range proxies {
		keepSet[proxy.ProxyAddress] = true
	}

	deleteList := []string{}
	for _, existingAddress := range existingProxies {
		if _, ok := keepSet[existingAddress]; !ok {
			deleteList = append(deleteList, existingAddress)
		}
	}

	for _, deleteAddress := range deleteList {
		err := con.client.deregisterServerProxy(masterBrokerAddress, deleteAddress)
		if err != nil {
			reqLogger.Error(err, "failed to deregister server proxy",
				"proxyAddress", deleteAddress,
				"Name", cr.ObjectMeta.Name,
				"ClusterName", cr.Spec.ClusterName)
		}
	}

	return nil
}

func (con *metaController) createCluster(reqLogger logr.Logger, masterBrokerAddress string, cr *undermoonv1alpha1.Undermoon) error {
	exists, err := con.client.clusterExists(masterBrokerAddress, cr.Spec.ClusterName)
	if err != nil {
		reqLogger.Error(err, "failed to check whether cluster exists",
			"Name", cr.ObjectMeta.Name,
			"ClusterName", cr.Spec.ClusterName)
		return err
	}

	if exists {
		return nil
	}

	err = con.client.createCluster(masterBrokerAddress, cr.Spec.ClusterName, int(cr.Spec.ChunkNumber))
	if err != nil {
		reqLogger.Error(err, "failed to create cluster",
			"Name", cr.ObjectMeta.Name,
			"ClusterName", cr.Spec.ClusterName)
		return err
	}
	return nil
}

func (con *metaController) changeNodeNumber(reqLogger logr.Logger, masterBrokerAddress string, cr *undermoonv1alpha1.Undermoon) error {
	chunkNumber := int(cr.Spec.ChunkNumber)
	clusterName := cr.Spec.ClusterName

	err := con.client.scaleNodes(masterBrokerAddress, clusterName, chunkNumber)
	if err != nil {
		if err == errMigrationRunning {
			return errRetryReconciliation
		}
		reqLogger.Error(err, "failed to scale nodes",
			"Name", cr.ObjectMeta.Name,
			"ClusterName", cr.Spec.ClusterName)
		return err
	}

	return nil
}

func (con *metaController) getClusterInfo(reqLogger logr.Logger, masterBrokerAddress string, cr *undermoonv1alpha1.Undermoon) (*clusterInfo, error) {
	info, err := con.client.getClusterInfo(masterBrokerAddress, cr.Spec.ClusterName)
	if err != nil {
		reqLogger.Error(err, "failed to get cluster info",
			"Name", cr.ObjectMeta.Name,
			"ClusterName", cr.Spec.ClusterName)
		return nil, err
	}
	return info, nil
}
