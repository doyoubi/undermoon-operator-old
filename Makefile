build:
	helm package helm/undermoon-operator
	helm package helm/undermoon-cluster

OPERATOR_HELM_VERSION="0.1.0"

install-helm-package:
	helm install example-operator "undermoon-operator-$(OPERATOR_HELM_VERSION).tgz"
	helm install example-undermoon "undermoon-cluster-$(OPERATOR_HELM_VERSION).tgz"

uninstall-helm-package:
	helm uninstall example-undermoon || true
	helm uninstall example-operator || true

lint:
	go fmt $(shell go list ./...)
	golint ./pkg/controller...
	golint ./test/e2e...
	# Helm Charts development:
	helm lint helm/undermoon-operator --strict
	helm lint helm/undermoon-cluster --strict

HELM_CHARTS_CRD_FILE=helm/undermoon-operator/templates/undermoon.operator.api_undermoons_crd.yaml
HELM_CHARTS_RBAC_FILE=helm/undermoon-operator/templates/operator-rbac.yaml

update-types:
	operator-sdk generate k8s
	operator-sdk generate crds
	# Update crd file in Helm Charts
	echo '# DO NOT MODIFY! This file is copied from deploy/crds/undermoon.operator.api_undermoons_crd.yaml' > $(HELM_CHARTS_CRD_FILE)
	cat deploy/crds/undermoon.operator.api_undermoons_crd.yaml >> $(HELM_CHARTS_CRD_FILE)
	echo '# DO NOT MODIFY! This file is generated from several files in deploy/' > $(HELM_CHARTS_RBAC_FILE)
	cat deploy/service_account.yaml >> $(HELM_CHARTS_RBAC_FILE)
	echo '---' >> $(HELM_CHARTS_RBAC_FILE)
	cat deploy/role.yaml >> $(HELM_CHARTS_RBAC_FILE)
	echo '---' >> $(HELM_CHARTS_RBAC_FILE)
	cat deploy/role_binding.yaml >> $(HELM_CHARTS_RBAC_FILE)

kind-env:
	# Run kind with image registry instead of:
	# kind create cluster
	# See https://kind.sigs.k8s.io/docs/user/local-registry/
	# When using image built locally, we need to run `docker run push localhost:5000/docker_image_name`.
	scripts/kind-with-registry.sh

list-images:
	curl http://localhost:5000/v2/_catalog

debug-build:
	operator-sdk build localhost:5000/undermoon-operator:v0.0.1
	docker push localhost:5000/undermoon-operator:v0.0.1

debug-run:
	kubectl create -f deploy/crds/undermoon.operator.api_undermoons_crd.yaml
	# run operator
	kubectl create -f deploy/service_account.yaml
	kubectl create -f deploy/role.yaml
	kubectl create -f deploy/role_binding.yaml
	kubectl create -f deploy/operator.yaml

debug-logs:
	./scripts/operator_logs.sh

debug-start:
	kubectl apply -f deploy/crds/undermoon.operator.api_v1alpha1_undermoon_cr.yaml

debug-stop:
	kubectl delete -f deploy/crds/undermoon.operator.api_v1alpha1_undermoon_cr.yaml || true
	kubectl delete -f deploy/crds/undermoon.operator.api_undermoons_crd.yaml || true
	kubectl delete -f deploy/operator.yaml || true
	kubectl delete -f deploy/role_binding.yaml || true
	kubectl delete -f deploy/role.yaml || true
	kubectl delete -f deploy/service_account.yaml || true

debug-edit:
	kubectl edit undermoon/example-undermoon

run-busybox:
	kubectl run -i --tty --rm debug-busybox --image=busybox --restart=Never -- sh

run-jq-curl:
	kubectl run -i --tty --rm debug-jq-curl --image=dwdraju/alpine-curl-jq --restart=Never -- sh

run-redis-cli:
	kubectl run -i --tty --rm debug-redis-cli --image=redis --restart=Never -- sh

e2e-test:
	kubectl create namespace e2etest || true
	operator-sdk test local --debug --operator-namespace e2etest ./test/e2e --go-test-flags "-v"

cleanup-e2e-test:
	kubectl delete namespace e2etest || true

.PHONY: build test lint update-types minikube-env debug-run debug-start debug-stop

