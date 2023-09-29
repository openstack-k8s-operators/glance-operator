# Glance per-tenant Quotas

`Glance` supports resource consumption quotas on tenants through the use of
`Keystone`’s unified limits functionality. Resource limits are registered in
Keystone with some default values, and may be overridden on a per-tenant
basis. When a resource consumption attempt is made in Glance, the current
consumption is computed and compared against the limit set in Keystone; the
request is denied if the user is over the specified limit.

## Enable Quotas in the glance-operator

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
the CR `spec` defines the following parameters:

```
spec
  ...
  ...
  quotas:
    imageSizeTotal: 1000
    imageStageTotal: 1000
    imageCountUpload: 100
    imageCountTotal: 100
```

The fields above are defined at the top level CR and applied by default to all
the `GlanceAPI` instances.
It's not possible setup or override those values for each GlanceAPI instances
independently.
The Glance upstream [documentation](https://docs.openstack.org/glance/latest/admin/quotas.html#configuring-glance-for-per-tenant-quotas)
covers this topic, and when `Quotas` are enabled as per snippet above, the
`00-config.conf` main file will append to the default section:

```
[DEFAULT]
use_keystone_limits = True
```

and it creates a populated `[oslo_limit]` section with all the information
required to access `Keystone`.
