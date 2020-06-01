lint:
	go fmt $(shell go list ./...)
	golint ./pkg/controller...

update-types:
	operator-sdk generate k8s
	operator-sdk generate crds

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

run-busybox:
	kubectl run -i --tty --rm debug --image=busybox --restart=Never -- sh

run-jq-curl:
	kubectl run -i --tty --rm debug --image=dwdraju/alpine-curl-jq --restart=Never -- sh

.PHONY: build test lint update-types minikube-env debug-run debug-start debug-stop

