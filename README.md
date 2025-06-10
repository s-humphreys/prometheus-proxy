# prometheus-proxy

## Overview

prometheus-proxy is designed to serve Prometheus requests on a Kubernetes cluster
which is using a remote Prometheus instance (most likely a managed Prometheus
e.g. Azure Managed Prometheus).

This has been created due the desire to keep in-cluster Prometheus requests as
simple as possible, in cases where the Prometheus instance is no longer accessible
within the cluster.

The prometheus-proxy receives unauthenticated requests from within the cluster and
updates them to include any required authenticaiton headers. It then hits the Prometheus
endpoint and returns the raw Prometheus response to the original caller.
