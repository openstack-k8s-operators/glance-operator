# Horizon integration

It is possible to upload images from the web UI provided by Horizon.
On systems where the value of `LimitRequestBody` is not explicitly specified in
an `httpd` configuration, the [default value](https://access.redhat.com/articles/6975397)
is 1GiB.
As a consequence, if the total size of the HTTP request body exceeds the 1
GiB limit, httpd returns the `413 Request Entity Too Large` error code.
While one possibility is to configure `LimitRequestBody` in Horizon httpd, the
recommended option is to enable `CORS`.
In general Horizon has different available options.

1. `off`: disables the ability to upload images via Horizon.
2. `legacy`: enables local file upload by piping the image file through the
   Horizon’s web server.
2. `direct` sends the image file directly from the web browser to Glance.

"direct" is the preferred mode, and it requires `CORS` support to be enabled on
the Glance API service.
`direct` mode has several benefits:

- Eliminates the 1GiB upload limit
- Reduces network hops (`browser -> Glance` instead of `browser -> Horizon -> Glance`)
- Prevents filling up Horizon's local Pod ephemeral space with temporary upload
  files
- Better performance for large image uploads

To enable `CORS` in glance, perform the following steps:

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
It is not required to add the following parameters:

```
max_age=3600
allow_methods=GET,POST,PUT,DELETE
```

because they represent the [default values](https://github.com/openstack/oslo.middleware/blob/master/oslo_middleware/cors.py#L61)
set through oslo.
