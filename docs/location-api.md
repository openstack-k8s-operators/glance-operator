# Glance Location API and Split Layout Migration Guide

## Overview

Starting with RHOSO 19, Nova and Cinder have adopted Glance's new location API,
which eliminates the security concerns that previously required the split API
deployment model. This document explains what the location API is, how it
resolves the previous security vulnerabilities, and provides guidance for
migrating from split to single layout deployments.

## What is the Location API?

The Glance location API is a new Glance feature that provides secure access
to image location data for authorized services like Nova and Cinder. This API
was developed to address the security vulnerability described in
[OSSN-0090](https://wiki.openstack.org/wiki/OSSN/OSSN-0090).

### Key Features

- **Secure location sharing**: Provides controlled access to image location
  data without exposing it to end users
- **Service authentication**: Only authenticated OpenStack services can access
  location information
- **Backward compatibility**: Works with existing backends and configurations

### Benefits Over Split Layout

1. **Simplified topology**: **Single** StatefulSet deployment reduces complexity
2. **Resource efficiency**: **Halves** PVC resource requirements (staging area storage)

## Security Background

### Previous Security Issue (OSSN-0090)

Before the location API, Glance deployments with shared storage backends (Ceph,
Swift, S3, Cinder) faced a security vulnerability where:

- Image location URLs could be exposed to end users
- Malicious users could potentially access or modify image data directly
- Services needed access to image locations for performance optimization

### Split Layout Solution

The split layout was implemented as a workaround with:

```ini
# External API (user-facing)
show_image_direct_url = False
show_multiple_locations = False

# Internal API (service-facing)
show_image_direct_url = True
show_multiple_locations = True
```

This created two separate API endpoints - one for users and one for internal
services.

### Location API Resolution

The new location API resolves this by eliminating the need to expose location
data in regular API calls and allowing a single API instance to serve both
users and services securely.

## Migration Guide

### Prerequisites

- RHOSO 19 or later
- Backup of current Glance configuration
- Understanding of current backend configuration

### Migration Steps

> **Important**: Existing split deployments cannot automatically migrate to
> single layout. Manual migration is required to preserve backward compatibility
> in a brownfield scenario.

#### Step 1: Plan the Migration

1. **Identify current deployment**:
    ```bash
    oc get glanceapi -n openstack
    ```

2. **Check backend configuration**:
    ```bash
    oc get glance -n openstack -o yaml | yq '.spec.customServiceConfig'
    ```

3. **List the deployed PVCs used for Glance staging area**:

    ```bash
    oc get pvc -n openstack -l service=glance
    ```

#### Step 2: Create New Single Layout API

1. **Create new GlanceAPI configuration** (`glance-single-api.yaml`):
    ```yaml
    spec:
      glance:
        template:
          glanceAPIs:
            default-single:
              replicas: 3
              customServiceConfig: |
                # Copy your existing backend configuration here,
                # for example
                [DEFAULT]
                enabled_backends = default_backend:swift
                [glance_store]
                default_backend = default_backend
                [default_backend]
                swift_store_create_container_on_put = True
                swift_store_auth_version = 3
                swift_store_auth_address = {{ .KeystoneInternalURL }}
                swift_store_endpoint_type = internalURL
                swift_store_user = service:glance
                swift_store_key = {{ .ServicePassword }}
                swift_store_region = {{ .Region }}
              storage:
                storageRequest: 30G  # Adjust as needed
    ```

> Note: `keystoneEndpoint` is not changed. The transition to the new API is
> performed once the new glanceAPI is up and running.


2. **Apply the new configuration**:
    ```bash
    oc patch openstackcontrolplane $(oc get oscp -o custom-columns=NAME:.metadata.name --no-headers) \
      --type=merge --patch-file=glance-single-api.yaml
    ```

3. **Wait for new API to be ready**:
    ```bash
    oc get glance -w
    ```

#### Step 3: Switch Keystone Endpoint

1. **Update keystone endpoint to point to new API**:
    ```bash
    oc patch openstackcontrolplane $(oc get oscp -o custom-columns=NAME:.metadata.name --no-headers) \
      --type=merge -p='{"spec": {"glance": {"template": {"keystoneEndpoint":"default-single"}}}}'
    ```

2. **Verify new endpoint registration**:
    ```bash
    oc exec -it openstackclient -- openstack endpoint list | grep image
    ```

#### Step 4: Test New Deployment

1. **Test image operations**:
    ```bash
    # List images
    oc exec -it openstackclient -- openstack image list

    # Upload test image
    oc exec -it openstackclient -- openstack image create \
      --container-format bare --disk-format raw --file /dev/null test-image

    # Delete test image
    oc exec -it openstackclient -- openstack image delete test-image
    ```

2. **Verify backend connectivity**:
    ```bash
    # Check Glance logs
    oc logs -n openstack -f -l service=glance
    ```

#### Step 5: Remove Old Split APIs

1. **Remove old split APIs**:
    ```bash
    # Remove external API
    oc patch openstackcontrolplane $(oc get oscp -o custom-columns=NAME:.metadata.name --no-headers) \
      --type=json -p='[{"op": "remove", "path": "/spec/glance/template/glanceAPIs/default"}]'
    ```

2. **Verify cleanup**:
    ```bash
    oc get glanceapi
    oc get statefulset -n openstack -l service=glance
    ```

#### Step 6: Manual PVC Cleanup

> **Critical**: The operator does not automatically delete PVCs. Manual cleanup is required.

1. **List Glance PVCs**:
    ```bash
    oc get pvc -n openstack | grep glance
    ```

2. **Identify PVCs from decommissioned APIs**:
    ```bash
    # Look for PVCs with old API names (e.g., glance-default-external, glance-default-internal)
    oc get pvc -n openstack -o wide | grep -E "(external|internal)"
    ```

3. **Delete old PVCs**:
    ```bash
    # Delete PVCs from old split APIs
    oc get pvc -l glanceAPI=glance-default-external -o custom-columns=NAME:.metadata.name --no-headers | xargs -n 1 oc delete pvc
    oc get pvc -l glanceAPI=glance-default-internal -o custom-columns=NAME:.metadata.name --no-headers | xargs -n 1 oc delete pvc
    ```

4. **Verify PVC cleanup**:
    ```bash
    oc get pvc -n openstack -l service=glance
    ```

### Storage Requirements Comparison

#### Before (Split Layout)
```
API Layout: split (2 pods per API)
Storage per API: 2 × (storageRequest + cacheSize) × replicas
Example: 2 × (30G + 20G) × 3 = 300G per API
```

#### After (Single Layout)
```
API Layout: single (1 pod per API)
Storage per API: 1 × (storageRequest + cacheSize) × replicas
Example: 1 × (30G + 20G) × 3 = 150G per API
```

**Result**: 50% reduction in storage requirements.

### Deprecation timeline: Warning Messages

Starting with RHOSO 19, using split layout will generate the following warning:

```
Warning: The GlanceAPI split layout is deprecated. It is recommended to remove this parameter and rely on the default single layout
```
