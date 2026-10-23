"""Build safe, predictable Terraform command lines for each stack."""


def command(stack, operation, variables):
    args = ["terraform", f"-chdir=infra/{stack}", operation, "-input=false"]
    if stack == "services":
        # Azure can race while creating Container Apps in the same VNet and
        # fail to acquire the shared Service Association Link lease.
        args.append("-parallelism=1")
    if operation == "apply":
        args.append("-auto-approve")
    args.append(f"-var-file={variables}")
    return args
