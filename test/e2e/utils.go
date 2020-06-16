package e2e

import (
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func waitForStatefulSet(t *testing.T, kubeclient kubernetes.Interface, namespace, name string, replicas int, retryInterval, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		ss, err := kubeclient.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s StatefulSet\n", name)
				return false, nil
			}
			return false, err
		}

		if int(ss.Status.ReadyReplicas) == replicas {
			return true, nil
		}
		t.Logf("Waiting for %s StatefulSet (%d/%d)\n", name, ss.Status.ReadyReplicas, replicas)
		return false, nil
	})

	if err != nil {
		return err
	}

	t.Logf("StatefulSet %s is ready\n", name)
	return nil
}

func waitForServiceEndpoints(t *testing.T, kubeclient kubernetes.Interface, namespace, name string, endpointNumber int, retryInterval, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		endpoints, err := kubeclient.CoreV1().Endpoints(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s Service\n", name)
				return false, nil
			}
			return false, err
		}

		addressNum := 0
		for _, subnet := range endpoints.Subsets {
			addressNum += len(subnet.Addresses)
		}

		if addressNum == endpointNumber {
			return true, nil
		}
		t.Logf("Waiting for %s Service (%d/%d)\n", name, addressNum, endpointNumber)
		return false, nil
	})

	if err != nil {
		return err
	}

	t.Logf("Service %s is ready\n", name)
	return nil
}
