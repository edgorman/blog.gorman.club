terraform_deployment(
    name="deployment",
    root_module="//infrastructure/env:mod",
)

terraform_backend(source="terraform.tfbackend")

terraform_var_files(sources=["terraform.tfvars"])
