.PHONY: help init

help: ## Shows the available commands
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf " %-20s %s\n", $$1, $$2}'

init: ## First-time bootstrap of the root project (local state, then migrated into GCS)
	@TF_VERSION=$$(grep 'required_version' providers.tf | cut -d '=' -f2 | tr -d ' "'); \
	mv providers.tf providers.tf.bak && \
	echo "terraform { required_version = \"$$TF_VERSION\" }" > providers.tf
	@terraform init
	@terraform apply -var-file=../config/root/terraform.tfvars
	@mv providers.tf.bak providers.tf
	@terraform init -migrate-state -force-copy -backend-config=../config/root/terraform.tfbackend
	@rm -f terraform.tfstate terraform.tfstate.backup
