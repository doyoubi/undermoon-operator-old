package e2e

import (
	goctx "context"
	"fmt"
	"github.com/pkg/errors"
	"testing"
	"time"

	"github.com/doyoubi/undermoon-operator/pkg/apis"
	operator "github.com/doyoubi/undermoon-operator/pkg/apis/undermoon/v1alpha1"
	umctrl "github.com/doyoubi/undermoon-operator/pkg/controller/undermoon"
	"github.com/go-redis/redis/v8"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

const testUndermoonName string = "example-undermoon-test"
const testClusterName string = "test-cluster-name"

func TestUndermoon(t *testing.T) {
	undermoonList := &operator.UndermoonList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, undermoonList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	// run subtests
	t.Run("undermoon-group", func(t *testing.T) {
		t.Run("Cluster", UndermoonCluster)
	})
}

func undermoonScaleTest(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}
	// create undermoon custom resource
	exampleUndermoon := &operator.Undermoon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testUndermoonName,
			Namespace: namespace,
		},
		Spec: operator.UndermoonSpec{
			ClusterName:              testClusterName,
			ChunkNumber:              1,
			MaxMemory:                10,
			UndermoonImage:           "localhost:5000/undermoon_test",
			UndermoonImagePullPolicy: corev1.PullAlways,
			RedisImage:               "redis:5.0.8",
		},
	}
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), exampleUndermoon, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	// wait for example-undermoon-test to reach 2 replicas
	storageStatefulSetName := umctrl.StorageStatefulSetName(testUndermoonName)
	err = waitForStatefulSet(t, f.KubeClient, namespace, storageStatefulSetName, 2, retryInterval, timeout)
	if err != nil {
		return err
	}

	// wait for service to have 2 endpoints
	storageServiceName := umctrl.StorageServiceName(testUndermoonName)
	err = waitForServiceEndpoints(t, f.KubeClient, namespace, storageServiceName, 2, retryInterval, timeout)
	if err != nil {
		return err
	}

	// wait for service to have 2 endpoints
	storagePublicServiceName := umctrl.StoragePublicServiceName(testUndermoonName)
	err = waitForServiceEndpoints(t, f.KubeClient, namespace, storagePublicServiceName, 2, retryInterval, timeout)
	if err != nil {
		return err
	}

	// Test the Redis service
	publicService, err := f.KubeClient.CoreV1().Services(namespace).Get(storagePublicServiceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	publicServiceIP := publicService.Spec.ClusterIP
	err = setKeys(t, publicServiceIP, []string{"a", "b"})
	if err != nil {
		return err
	}
	err = getKeys(t, publicServiceIP, []string{"a", "b"})
	if err != nil {
		return err
	}

	// scale up to 2 chunks and 4 replicas
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: testUndermoonName, Namespace: namespace}, exampleUndermoon)
	if err != nil {
		return err
	}
	exampleUndermoon.Spec.ChunkNumber = 2
	err = f.Client.Update(goctx.TODO(), exampleUndermoon)
	if err != nil {
		return err
	}

	// wait for example-undermoon-test to reach 4 replicas
	err = waitForStatefulSet(t, f.KubeClient, namespace, storageStatefulSetName, 4, retryInterval, timeout)
	if err != nil {
		return err
	}

	// wait for service to have 4 endpoints
	err = waitForServiceEndpoints(t, f.KubeClient, namespace, storageServiceName, 4, retryInterval, timeout)
	if err != nil {
		return err
	}

	// wait for public service to have 4 endpoints
	err = waitForServiceEndpoints(t, f.KubeClient, namespace, storagePublicServiceName, 4, retryInterval, timeout)
	if err != nil {
		return err
	}

	// Test the Redis service
	err = setKeys(t, publicServiceIP, []string{"c", "d"})
	if err != nil {
		return err
	}
	err = getKeys(t, publicServiceIP, []string{"a", "b", "c", "d"})
	if err != nil {
		return err
	}

	// scale down to 1 chunks and 2 replicas
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: testUndermoonName, Namespace: namespace}, exampleUndermoon)
	if err != nil {
		return err
	}
	exampleUndermoon.Spec.ChunkNumber = 1
	err = f.Client.Update(goctx.TODO(), exampleUndermoon)
	if err != nil {
		return err
	}

	// wait for public service to have 2 endpoints
	err = waitForServiceEndpoints(t, f.KubeClient, namespace, storagePublicServiceName, 2, retryInterval, timeout)
	if err != nil {
		return err
	}

	// wait for service to have 2 endpoints
	err = waitForServiceEndpoints(t, f.KubeClient, namespace, storageServiceName, 2, retryInterval, timeout)
	if err != nil {
		return err
	}

	// wait for example-undermoon-test to reach 2 replicas
	err = waitForStatefulSet(t, f.KubeClient, namespace, storageStatefulSetName, 2, retryInterval, timeout)
	if err != nil {
		return err
	}

	// Test the Redis service
	err = setKeys(t, publicServiceIP, []string{"e", "f"})
	if err != nil {
		return err
	}
	err = getKeys(t, publicServiceIP, []string{"a", "b", "c", "d", "e", "f"})
	if err != nil {
		return err
	}

	return nil
}

func UndermoonCluster(t *testing.T) {
	t.Parallel()
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	// get global framework variables
	f := framework.Global
	// wait for undermoon-operator to be ready
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "undermoon-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	if err = undermoonScaleTest(t, f, ctx); err != nil {
		t.Fatal(err)
	}
}

func setKeys(t *testing.T, publicServiceIP string, keys []string) error {
	address := fmt.Sprintf("%s:%d", publicServiceIP, umctrl.ServerProxyPort)
	client := redis.NewClient(&redis.Options{
		Addr: address,
	})
	for _, key := range keys {
		value := fmt.Sprintf("%s:value", key)
		_, err := client.Set(goctx.Background(), key, value, time.Minute*5).Result()
		if err != nil {
			return nil
		}
	}
	return nil
}

func getKeys(t *testing.T, publicServiceIP string, keys []string) error {
	address := fmt.Sprintf("%s:%d", publicServiceIP, umctrl.ServerProxyPort)
	client := redis.NewClient(&redis.Options{
		Addr: address,
	})
	for _, key := range keys {
		v, err := client.Get(goctx.Background(), "a").Result()
		if err != nil {
			return nil
		}
		expectedValue := fmt.Sprintf("%s:value", key)
		if v != expectedValue {
			return errors.Errorf("value not equal: %s != %s", v, expectedValue)
		}
	}
	return nil
}
