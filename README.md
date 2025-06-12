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

### Configuration

The following environment variables are required to be set prior to running the service:
- `PROMETHEUS_URL` (required) - The URL for the Prometheus instance which requires proxying to.
- `LOG_LEVEL` (optional) - The logging level. One of `INFO`/`DEBUG`/`ERROR`/`WARN`.

In addition to the above, configuration for one of the supported authentication methods must
also be in place:
- [Azure](#azure)

### Azure

#### Prerequisites

The proxy supports using either an App Registration (with client secret), or a User-Assigned
Managed Identity using workload identity.

> If using Azure Managed Prometheus, the identity you use must have the `Monitoring Data
Reader` role assigned to the Azure Monitor Workspace, or the subscription it resides in.

#### Authentication

##### App Registration

If using an App Registration, you must set the following environment variables prior to
running the service:
- `AZURE_TENANT_ID` (required) - the Azure tenant ID.
- `AZURE_CLIENT_ID` (required) - the client ID of the App Registration.
- `AZURE_CLIENT_SECRET` (required) - the client secret of the App Registration.

##### User-Assigned Managed Identity (workload identity)

If using a User-Assigned Managed Identity, you must ensure the application is running using a
Kubernetes service account bound to the Managed Identity and a valid Federated Credential is in
place. The service will use the auto-mounted credentials which AKS adds to the Pod.

The following environment variables must be set prior to running the service:
- `AZURE_TENANT_ID` (required) - the Azure tenant ID.
