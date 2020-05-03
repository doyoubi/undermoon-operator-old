update-types:
	operator-sdk generate k8s
	operator-sdk generate crds

.PHONY: build test lint update-types

