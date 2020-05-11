lint:
	go fmt $(shell go list ./...)
	golint ./pkg/controller...

update-types:
	operator-sdk generate k8s
	operator-sdk generate crds

kind-env:
	kind create cluster

debug-run:
	kubectl create -f deploy/crds/undermoon.operator.api_undermoons_crd.yaml
	export OPERATOR_NAME=undermoon-operator
	operator-sdk run --local --watch-namespace=default

debug-start:
	kubectl apply -f deploy/crds/undermoon.operator.api_v1alpha1_undermoon_cr.yaml

debug-stop:
	kubectl delete -f deploy/crds/undermoon.operator.api_v1alpha1_undermoon_cr.yaml || true
	kubectl delete -f deploy/crds/undermoon.operator.api_undermoons_crd.yaml || true
	kubectl delete -f deploy/operator.yaml || true
	kubectl delete -f deploy/role_binding.yaml || true
	kubectl delete -f deploy/role.yaml || true
	kubectl delete -f deploy/service_account.yaml || true

.PHONY: build test lint update-types minikube-env debug-run debug-start debug-stop

