# prometheus-proxy

## Overview

prometheus-proxy is designed to serve Prometheus requests on a Kubernetes cluster
which is using a remote Prometheus instance (most likely a managed Prometheus
e.g. Azure Managed Prometheus).

This has been created due the desire to keep in-cluster Prometheus requests as
simple as possible, in cases where the Prometheus instance is no longer accessible
within the cluster.

The prometheus-proxy receives unauthenticated requests from within the cluster and
updates them to include any required authentication headers. It then hits the Prometheus
endpoint and returns the raw Prometheus response to the original caller.

## Deployment

Currently the only supported authentication methods is [Azure](#azure), which handles requests
to Azure Managed Prometheus. Ensure you fulfil the pre-requisites.

### Usage

```sh
Usage:
  run [flags]

Flags:
      --azure-client-id string       The Azure Client ID to use for authentication
      --azure-client-secret string   The Azure Client Secret to use for authentication (if not provided, will use Managed Identity)
      --azure-tenant-id string       The Azure Tenant ID to use for authentication
  -h, --help                         help for run
      --log-level string             The log level to use (default "INFO")
      --port int                     The port to run the proxy on (default 9090)
      --prometheus-url string        The URL of the Prometheus instance to proxy requests to
```

### Azure

#### Prerequisites

The proxy supports using either an App Registration (with client secret), or a User-Assigned
Managed Identity using workload identity.

> If using Azure Managed Prometheus, the identity you use must have the `Monitoring Data
Reader` role assigned to the Azure Monitor Workspace, or the subscription it resides in.

#### Authentication

##### App Registration

If using an App Registration, you must set the following args must be set prior whilst running the service:
- `--azure-tenant-id` (required) - the Azure tenant ID.
- `--azure-client-id` (required) - the client ID of the App Registration.
- `--azure-client-secret` (required) - the client secret of the App Registration.

##### User-Assigned Managed Identity (workload identity)

If using a User-Assigned Managed Identity, you must ensure the application is running using a
Kubernetes service account bound to the Managed Identity and a valid Federated Credential is in
place. The service will use the auto-mounted credentials which AKS adds to the Pod.

The following args must be set whilst running the service:
- `--azure-tenant-id` (required) - the Azure tenant ID.
- `--azure-client-id` (required) - the client ID of the App Registration. You can
use the auto-injected AKS environment variable to set this arg, like `--azure-client-id=$(AZURE_CLIENT_ID)`.
