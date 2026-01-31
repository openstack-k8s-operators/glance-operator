# Horizon integration

It is possible to upload images from the web UI provided by Horizon.
On systems where the value of `LimitRequestBody` is not explicitly specified in
an `httpd` configuration, the [default value](https://access.redhat.com/articles/6975397)
is 1GiB.
As a consequence, if the total size of the HTTP request body exceeds the 1
GiB limit, httpd returns the `413 Request Entity Too Large` error code.
While one possibility is to configure `LimitRequestBody` in Horizon httpd, the
recommended option is to enable `CORS`.
Horizon has three available upload modes.

1. `off`: disables the ability to upload images via Horizon.
2. `legacy`: enables local file upload by piping the image file through the
   Horizon’s web server.
3. `direct` sends the image file directly from the web browser to Glance.

"direct" is the preferred mode, and it requires `CORS` support to be enabled on
the Glance API service.

`direct` mode has several benefits:

- Eliminates the 1GiB upload limit
- Reduces network hops (`browser -> Glance` instead of `browser -> Horizon -> Glance`)
- Prevents filling up Horizon's local Pod ephemeral space with temporary upload
  files
- Better performance for large image uploads

To enable `CORS` in Glance, perform the following steps:

1. Get the Horizon `Route` associated with the deployment:

```bash
$ oc get route horizon -o custom-columns=HOST:.spec.host --no-headers
horizon-openstack.apps.ocp.openstack.lab
```
2. Edit the `OpenStackControlPlane` and add the `CORS` section in the Glance
   configuration:

```yaml
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack
spec:
...
  glance:
      apiOverrides:
        default:
          route:
            metadata:
              annotations:
                api.glance.openstack.org/timeout: 60s
                haproxy.router.openshift.io/timeout: 60s
      enabled: true
      template:
        apiTimeout: 60
        customServiceConfig: |
          [cors]
          allowed_origin=https://horizon-openstack.apps.ocp.openstack.lab
```

**Note:**
You don't need to add the following parameters:

```
max_age=3600
allow_methods=GET,POST,PUT,DELETE
```

because they represent the [default values](https://github.com/openstack/oslo.middleware/blob/master/oslo_middleware/cors.py#L61)
set through oslo.

## Multi-Region Setup

In multi-region deployments where multiple GlanceAPI instances are deployed in
each region, there's usually a single centralized Horizon instance.
In particular:
- Multiple GlanceAPI instances are deployed across different regions/edge sites
- A single centralized Horizon dashboard serves all regions
- Direct image upload mode is required for performance and scalability
In such deployments, the `HORIZON_IMAGES_UPLOAD_MODE` variable **must be set to
`direct`** mode. The `legacy` mode cannot be used because it routes all image
upload traffic through the central Horizon instance, creating unnecessary
network overhead and defeating the performance benefits of distributed edge
sites.

This scenario makes CORS configuration more complex. The CORS section cannot be
automatically configured in environments where the local Horizon instance is
not deployed in the control plane, because we can't rely on the `glance-operator`
discovery.

### CORS Configuration for Edge Sites

When using direct upload mode across multiple regions, you'll encounter
Cross-Origin Resource Sharing (CORS) restrictions. The browser will block
cross-origin requests from the Horizon domain to edge GlanceAPI endpoints
unless explicitly configured.

According to the [OpenStack Horizon
documentation](https://docs.openstack.org/horizon/latest/configuration/settings.html#images-upload-mode),
Glance services at edge sites must inform the browser via HTTP headers that
they accept requests from the Horizon origin domain.

### Configuration Steps

1. **Identify the Horizon endpoint** that will access the edge GlanceAPI
   services.
   In the `central` openstack control plane:

```bash
# Get all the Horizon routes that will need access to edge Glance services
$ oc get route horizon -o custom-columns=HOST:.spec.host --no-headers
horizon-openstack.apps.ocp.openstack.lab
```

2. **Configure CORS in the edge region GlanceAPI instances** by adding the
   following to the `customServiceConfig` section:

```yaml
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack-region-2
spec:
...
  glance:
      enabled: true
      template:
        customServiceConfig: |
          [cors]
          allowed_origin=https://horizon-openstack.apps.ocp.openstack.lab:443,https://horizon-backup.apps.ocp.openstack.lab:443
          max_age=3600
          allow_methods=GET,POST,PUT,DELETE
          allow_headers=X-Custom-Header,Content-Type,Authorization
          expose_headers=X-Custom-Header
```

### Configuration Parameters Explained

- **allowed_origin**: Comma-separated list of all Horizon endpoints that need
  access to this GlanceAPI instance. Include the full URL with protocol and
  port.
- **max_age**: Time in seconds that browsers can cache CORS preflight responses
  (3600 = 1 hour).
- **allow_methods**: HTTP methods permitted for cross-origin requests. Include
  all methods used by Horizon for image operations.
- **allow_headers**: Custom headers that Horizon may send with requests.
- **expose_headers**: Headers that the browser is allowed to access in the
  response.

### Troubleshooting CORS Issues

If image uploads from Horizon to edge sites fail, check for the following:

1. **Browser Console Errors**: Look for CORS-related errors in the browser
   developer console:
   ```
   Access to XMLHttpRequest at
   'https://glance-edge.apps.openstack-region-2.openstack.lab' from origin
   'https://horizon-openstack.apps.ocp.openstack.lab' has been blocked by CORS policy
   ```

2. **Verify CORS Headers**: Use browser developer tools to check that the
   Glance API is returning proper CORS headers:
   ```
   Access-Control-Allow-Origin: https://horizon-openstack.apps.ocp.openstack.lab
   Access-Control-Allow-Methods: GET, POST, PUT, DELETE
   ```

3. **Test with curl**: Verify CORS configuration by sending a preflight
   request:

   ```bash
   curl -H "Origin: https://horizon-openstack.apps.ocp.openstack.lab" \
        -H "Access-Control-Request-Method: POST" \
        -H "Access-Control-Request-Headers: Content-Type" \
        -X OPTIONS \
        https://glance-edge.apps.openstack-region-2.openstack.lab/v2/images
   ```
