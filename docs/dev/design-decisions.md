# Design decisions

## Umbrella Glance Service (Split API deployment)

As mentioned in [OSSN-0090](https://wiki.openstack.org/wiki/OSSN/OSSN-0090),
when deploying Glance in a popular configuration where Glance shares a
common storage backend with Nova and/or Cinder, it is possible to open some
known attack vectors by which malicious data modification can occur. If you
choose to deploy a glance operator with Ceph as a backend then by default you
will get a split API (Internal Vs External Glance API) deployed.

- A ``user facing`` glance-api service, accessible via the Public and Admin
keystone endpoints.
- An ``internal facing only`` service, accessible via
the Internal keystone endpoint.

The user facing service is configured to not expose image locations, namely by
setting the following options in glance-api.conf:

```editorconfig
[DEFAULT]
show_image_direct_url = False
show_multiple_locations = False
```

The internal service, operating on a different port (e.g. 9293), is configured
identically to the public facing service, except for the following:

```editorconfig
[DEFAULT]
show_image_direct_url = True
show_multiple_locations = True
```

OpenStack services that use glance (cinder and nova) is configured to access
it via the new internal service. That way both cinder and nova will have
access to the image location data.
