# Glance HTTPD Configuration Overrides

The glance-operator provides mechanisms to customize the Apache HTTPD server
configuration through the use of custom configuration files. This feature
leverages the
[ExtraMounts](https://github.com/openstack-k8s-operators/dev-docs/blob/main/extra_mounts.md)
functionality to mount custom HTTPD configuration files into the Glance
deployment.

## How It Works

1. **Custom Configuration Files**: Create HTTPD configuration files with your
   custom settings
2. **ConfigMap**: Create ConfigMaps from files containing the overrides
3. **OpenStackControlPlane Patch**: Patch the control plane to mount the
   generated ConfigMap into Glance containers. The HTTPD configuration
   automatically includes files mounted to `/etc/httpd/conf_custom/*.conf`


### Step 1: Create Custom HTTPD Configuration

Create your custom HTTPD configuration file(s). The filename must start with
`httpd_custom` to be automatically included by the base HTTPD configuration.

Example (`httpd_custom_timeout.conf`):
```apache
# Custom timeout settings for Glance
Timeout 300
KeepAliveTimeout 15
```

### Step 2. Create a ConfigMap

Create a Kubernetes ConfigMap containing your custom configuration files:

```bash
oc create configmap httpd-overrides --from-file=httpd_custom_timeout.conf
```

It is possible to add multiple configuration files containing dedicated
configuration directives:

```bash
oc create configmap httpd-overrides \
  --from-file=httpd_custom_timeout.conf \
  --from-file=httpd_custom_security.conf \
  --from-file=httpd_custom_logging.conf
```

The following example is based on a single customization file and demonstrates
how to set a custom `Timeout` parameter.

### Step 3: Configure ExtraMounts in the OpenStackControlPlane

Update your `OpenStackControlPlane` resource to include the custom HTTPD
configuration files using `extraMounts`. The simplest approach is to mount
the entire ConfigMap to the target `/etc/httpd/conf_custom` mount point:

```yaml
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack
spec:
  glance:
    enabled: true
    template:
      glanceAPIs:
        extraMounts:
        - extraVol:
          - extraVolType: httpd-overrides
            mounts:
            - mountPath: /etc/httpd/conf_custom
              name: httpd-overrides
              readOnly: true
            volumes:
            - configMap:
                name: httpd-overrides
              name: httpd-overrides
```

#### Selective File Mount (Advanced)

If you need to mount only specific files from the ConfigMap or customize their
mount paths, you can use the selective mount approach:

```yaml
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack
spec:
  glance:
    enabled: true
    template:
      glanceAPIs:
        extraMounts:
        - extraVol:
          - extraVolType: httpd-overrides
            mounts:
            - mountPath: /etc/httpd/conf_custom/httpd_custom_timeout.conf
              name: httpd-overrides
              readOnly: true
              subPath: httpd_custom_timeout.conf
            - mountPath: /etc/httpd/conf_custom/httpd_custom_security.conf
              name: httpd-overrides
              readOnly: true
              subPath: httpd_custom_security.conf
            - mountPath: /etc/httpd/conf_custom/httpd_custom_logging.conf
              name: httpd-overrides
              readOnly: true
              subPath: httpd_custom_logging.conf
            volumes:
            - configMap:
                name: httpd-overrides
              name: httpd-overrides
```

All the specified files are mounted and loaded by httpd during the Pod
bootstrap process.

## ExtraMounts Configuration Details

The `extraMounts` feature uses the following key components:

- **extraVolType**: Set to `httpd-overrides` to indicate the type of volume
  being mounted
- **mountPath**: The full path where the configuration file will be mounted
  inside the container (`/etc/httpd/conf_custom/`)
- **subPath**: The specific file from the ConfigMap to mount
- **readOnly**: Set to `true` to mount the configuration files as read-only
- **volumes**: References the ConfigMap containing the configuration files

The HTTPD overrides feature:

1. **Leverages ExtraMounts**: Both use the `extraMounts` mechanism to inject
   files into pods
2. **Requires Specific Mount Paths**:
   - HTTPD overrides mount to `/etc/httpd/conf_custom/` as specified in the
     httpd.conf `IncludeOptional` directive

## Common Use Cases

- **Timeout Adjustments**: Modify request timeout values for specific environments
- **Security Headers**: Add custom security headers or configurations
- **Logging**: Customize Apache logging configuration
- **Performance Tuning**: Adjust worker processes, connection limits, etc.

## Verification

After deploying your custom HTTPD configuration, you can verify that the
settings have been properly applied:

### 1. Find the Glance Pod

First, identify the running Glance pod:

```bash
$ oc get pods -l service=glance
```

### 2. Verify Configuration Loading

Connect to the Glance Pod and check that your custom configuration has been
loaded:

```bash
# Replace <Glance-pod-name> with the actual pod name from step 1
oc rsh -c glance-httpd <glance-pod-name>
# Inside the pod, dump the HTTPD configuration and check for your custom settings
httpd -D DUMP_CONFIG | grep -i timeout
```

For the `httpd_custom_timeout.conf` example, you should see output similar to:

```
Timeout 120
```

### 3. Additional Verification Commands

You can also verify other aspects of the configuration:

```bash
# Check all loaded configuration files
$ httpd -D DUMP_INCLUDES
Included configuration files:
  (*) /etc/httpd/conf/httpd.conf
    (14) /etc/httpd/conf.modules.d/00-base.conf
    (14) /etc/httpd/conf.modules.d/00-brotli.conf
    (14) /etc/httpd/conf.modules.d/00-dav.conf
    (14) /etc/httpd/conf.modules.d/00-mpm.conf
    (14) /etc/httpd/conf.modules.d/00-optional.conf
    (14) /etc/httpd/conf.modules.d/00-proxy.conf
    (14) /etc/httpd/conf.modules.d/00-ssl.conf
    (14) /etc/httpd/conf.modules.d/00-systemd.conf
    (14) /etc/httpd/conf.modules.d/01-cgi.conf
    (14) /etc/httpd/conf.modules.d/10-wsgi-python3.conf
    (25) /etc/httpd/conf.d/10-glance-wsgi.conf
      (40) /etc/httpd/conf_custom/httpd_custom_timeout.conf
```

### 4. Verify ConfigMap Mount via ExtraMounts

Outside the pod, you can also verify that the ConfigMap is properly mounted
through extraMounts:

```bash
# Check that the ConfigMap exists
oc get configmap httpd-overrides -o yaml
# Verify the mount in the pod description
oc describe pod <glance-pod-name>
```

## Deploy the Sample

The glance-operator repository includes a sample that can be used to deploy
glance with httpd overrides (it set a particular Timeout to 120s). This sample
is provided as a working reference example and is not necessarily meant to
serve as a deployment recommendation for production environments.

If you're using
[`install_yamls`](https://github.com/openstack-k8s-operators/install_yamls) and
already have CRC (Code Ready Containers) running, you can deploy the httpd
overrides example with the following steps:

```bash
# Navigate to the install_yamls directory
$ cd install_yamls
# Set up the CRC storage and deploy OpenStack Catalog
$ make crc_storage openstack
# Deploy OpenStack operators
$ make openstack_init
# Generate the OpenStack deployment file
$ oc kustomize . > ~/openstack-deployment.yaml
# Set the path to the deployment file
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
```

This will create the necessary ConfigMap and a deployable OpenStackControlPlane
yaml with the custom timeout configuration applied.
