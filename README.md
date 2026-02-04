# Kubernetes Helm plugin

The idea was base on CI/CD pipelines where multiple jobs may attempt to deploy the same Helm release concurrently or some one may stop the job while helm upgrade is in progress. After that the release may get into failed state and block further upgrades.

We need a way to ensure that only one helm upgrade command is executed for a given release, and if lock acquisition times out, we want to check if the release is in failed state and roll it back automatically.

The error message we want to handle is:

```shell
Error: UPGRADE FAILED: another operation (install/upgrade/rollback) is in progress
```

## Installation

```shell
helm plugin install https://github.com/sergelogvinov/helm-lock
```

## Usage

### Basic Syntax

```shell
helm lock <HELM_COMMAND> <RELEASE_NAME> [CHART] [FLAGS]
```

### Common Examples

**Upgrade a release with lock protection:**

```shell
helm lock upgrade my-release ./my-chart
```

**Install a release with custom timeout:**

```shell
helm lock install my-release ./my-chart --lock-timeout 5m
```

**Upgrade with Helm flags:**

```shell
helm lock upgrade my-release ./my-chart --namespace production --set image.tag=v1.2.3
```

### How it works

1. **Lock Acquisition**: The plugin uses Kubernetes leader election to acquire a distributed lock named `helm-lock-<release-name>`
2. **Release Status Check**: Checks if the Helm release is in a healthy state (`deployed` or `unknown`)
3. **Automatic Rollback**: If the release is in a failed state, performs an automatic rollback before executing the command
4. **Command Execution**: Executes the original Helm command with all provided arguments and flags
5. **Lock Release**: Automatically releases the lock when the operation completes

### Configuration Options

| Flag | Default | Description |
|------|---------|-------------|
| `--lock-timeout` | `10m` | Maximum time to wait for lock acquisition |
| `--namespace` | `default` | Kubernetes namespace (inherited from Helm) |
| `--debug` | `false` | Enable debug output (inherited from Helm) |

### Supported Helm Commands

The plugin supports wrapping any Helm command, but is most useful with:

- `upgrade` - Most common use case for preventing concurrent deployments
- `install` - Prevents race conditions during initial deployment

### Examples for CI/CD

**GitLab CI:**
```yaml
deploy:
  script:
    - helm lock upgrade $RELEASE_NAME ./chart --lock-timeout 5m --namespace $NAMESPACE
```

**GitHub Actions:**
```yaml
- name: Deploy with Helm Lock
  run: |
    helm lock upgrade ${{ env.RELEASE_NAME }} ./chart \
      --lock-timeout 5m \
      --namespace ${{ env.NAMESPACE }} \
      --set image.tag=${{ github.sha }}
```

**Jenkins:**
```groovy
sh "helm lock upgrade ${RELEASE_NAME} ./chart --lock-timeout 5m --namespace ${NAMESPACE}"
```

## Requirements

- Helm v3+
- Kubernetes cluster access
