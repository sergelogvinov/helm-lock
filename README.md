# Kubernetes Helm plugin

The idea was base on CI/CD pipelines where multiple jobs may attempt to deploy the same Helm release concurrently or some one may stop the job while helm upgrade is in progress. After that the release may get into failed state and block further upgrades.

We need a way to ensure that only one helm upgrade command is executed for a given release, and if lock acquisition times out, we want to check if the release is in failed state and roll it back automatically.
