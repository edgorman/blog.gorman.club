terraform_deployment(
    name="deployment",
    root_module="//infrastructure/env:mod",
)

terraform_backend(name="backend", source="terraform.tfbackend")

terraform_var_files(name="vars", sources=["terraform.tfvars"])
