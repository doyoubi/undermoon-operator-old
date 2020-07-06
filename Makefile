OPERATOR_VERSION="v0.0.1"
OPERATOR_HELM_VERSION="0.1.0"
CHECKER_HELM_VERSION="0.1.0"

build: build-image build-helm
	echo "build done"

build-image:
	echo "building undermoon-operator:$(OPERATOR_VERSION)"
	operator-sdk build undermoon-operator:$(OPERATOR_VERSION)

build-helm:
	helm package helm/undermoon-operator
	helm package helm/undermoon-cluster

install-helm-package:
	helm package helm/undermoon-operator
	helm package helm/undermoon-cluster
	helm install \
		--set "image.operatorImage=localhost:5000/undermoon-operator:$(OPERATOR_VERSION)" \
		example-operator "undermoon-operator-$(OPERATOR_HELM_VERSION).tgz"
	helm install \
		--set "image.undermoonImage=localhost:5000/undermoon_test" \
		example-undermoon "undermoon-cluster-$(OPERATOR_HELM_VERSION).tgz"

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
	helm lint chaostest/undermoon-checker --strict

lint-chaostest-script:
	# pip install -r chaostest/ctrl_requirements.txt
	black chaostest
	pylint --errors-only chaostest

OPERATOR_CRD_FILE=deploy/crds/undermoon.operator.api_undermoons_crd.yaml
OPERATOR_CR_FILE=deploy/crds/undermoon.operator.api_v1alpha1_undermoon_cr.yaml
HELM_CHARTS_CRD_FILE=helm/undermoon-operator/templates/undermoon.operator.api_undermoons_crd.yaml
HELM_CHARTS_RBAC_FILE=helm/undermoon-operator/templates/operator-rbac.yaml

update-types:
	operator-sdk generate k8s
	operator-sdk generate crds
	# Update crd file in Helm Charts
	echo '# DO NOT MODIFY! This file is copied from $(OPERATOR_CRD_FILE)' > $(HELM_CHARTS_CRD_FILE)
	cat $(OPERATOR_CRD_FILE) >> $(HELM_CHARTS_CRD_FILE)
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
	operator-sdk build localhost:5000/undermoon-operator:$(OPERATOR_VERSION)
	docker push localhost:5000/undermoon-operator:$(OPERATOR_VERSION)

debug-run:
	kubectl create -f $(OPERATOR_CRD_FILE)
	# run operator
	kubectl create -f deploy/service_account.yaml
	kubectl create -f deploy/role.yaml
	kubectl create -f deploy/role_binding.yaml
	kubectl create -f deploy/operator.yaml

debug-logs:
	./scripts/operator_logs.sh

debug-start:
	kubectl apply -f $(OPERATOR_CR_FILE)

debug-stop:
	kubectl delete -f $(OPERATOR_CR_FILE) || true
	kubectl delete -f $(OPERATOR_CRD_FILE) || true
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
	kubectl run -i --tty --rm debug-redis-cli --image=redis --restart=Never -- bash

e2e-test:
	kubectl create namespace e2etest || true
	operator-sdk test local --debug --operator-namespace e2etest ./test/e2e --go-test-flags "-v"

cleanup-e2e-test:
	kubectl delete namespace e2etest || true

docker-build-checker-image:
	docker image build -f chaostest/Dockerfile -t undermoon_checker .
	docker tag undermoon_checker localhost:5000/undermoon_checker
	docker push localhost:5000/undermoon_checker

install-undermoon-checker:
	helm package chaostest/undermoon-checker
	helm install example-checker "undermoon-checker-$(CHECKER_HELM_VERSION).tgz"

install-undermoon-chaos-checker:
	helm package chaostest/undermoon-checker
	helm install --set chaos=true example-checker "undermoon-checker-$(CHECKER_HELM_VERSION).tgz"

uninstall-undermoon-checker:
	helm uninstall example-checker

checker-logs:
	./scripts/checker_logs.sh

test-ctrl:
	python chaostest/test_controller.py example-undermoon disable-killing

test-chaos-ctrl:
	python chaostest/test_controller.py example-undermoon enable-killing

.PHONY: build test lint update-types minikube-env debug-run debug-start debug-stop

