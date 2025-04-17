# Glance per-tenant Quotas

`Glance` supports resource consumption quotas on tenants through the use of
`Keystone`’s unified limits functionality. Resource limits are registered in
Keystone with some default values, and may be overridden on a per-tenant
basis. When a resource consumption attempt is made in Glance, the current
consumption is computed and compared against the limit set in Keystone; the
request is denied if the user is over the specified limit.

## Enable Quotas in the Glance CR

The `glance-operator` and the current `API` implementation supports four types
of quotas that, during the `GlanceAPI` deployment, are updated in `Keystone`:

* `image_size_total`: maximum amount of storage (in MiB) that the tenant may
   consume across all of their active images
* `image_stage_total`: defines the total amount of staging space that may be used
* `image_count_total`: maximum number of image objects that the user may have
* `image_count_uploading`: number of parallel upload operations that can be in
   progress at any single point

As per the
[example](https://github.com/openstack-k8s-operators/glance-operator/tree/main/config/samples/quotas/glance_v1beta_glance_quotas.yaml),
the top level Glance CR `spec` defines the following parameters:

```
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack
spec:
  glance:
    template:
      ...
      ...
      quotas:
        imageSizeTotal: 10
        imageStageTotal: 10
        imageCountUpload: 10
        imageCountTotal: 10
```

The fields above are defined at the top level CR and applied by default to all
the `GlanceAPI` instances.
It's not possible setup or override those values for each GlanceAPI instances
independently.
The Glance upstream [documentation](https://docs.openstack.org/glance/latest/admin/quotas.html#configuring-glance-for-per-tenant-quotas)
covers this topic, and when `Quotas` are enabled as per example above, the
`00-config.conf` config file is updated with the relevant config options in both
the `[DEFAULT]` section and the `[oslo_limit]` one:

```
[DEFAULT]
use_keystone_limits = True
...
...
[oslo_limit]
auth_url= {{ .KeystoneInternal }}
auth_type = password
username={{ .ServiceUser }}
password = {{ .ServicePassword }}
system_scope = all
user_domain_id = default
endpoint_id = {{ .EndpointID }}
```

### Note

The configuration above registers the user-defined limits and updates the
`Glance` service settings accordingly.
However, this alone may not be sufficient for `Glance` to function correctly
via the CLI.

To ensure proper access, the `glance` user must be granted the `reader` role.
Assign the role using the following command:

```bash
openstack role add --user glance --system all reader
```

Once the role is assigned, the user will have the necessary authorization in
Keystone. Without it, you might encounter a `500 Internal Server Error` when
running CLI commands:

```bash
$ glance usage
HTTP 500 Internal Server Error: The server has either erred or is incapable of performing the requested operation.
```

After assigning the role:

```bash
$ openstack role add --user glance --system all reader
$ glance usage
+-----------------------+-------+-------+
| Quota                 | Limit | Usage |
+-----------------------+-------+-------+
| image_size_total      | 10    | 0     |
| image_stage_total     | 10    | 0     |
| image_count_total     | 10    | 0     |
| image_count_uploading | 10    | 0     |
+-----------------------+-------+-------+
```
