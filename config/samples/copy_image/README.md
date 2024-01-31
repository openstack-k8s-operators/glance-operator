# Configuring copy-image import method

For the `copy-image` method, make sure that `copy-image` is included in
the list specified by your `enabled_import_methods` setting as well as
you have multiple glance backends configured in your environment. By
default, only the image owner or administrator can copy existing images
to newly added stores.

To  allow `copy-image` operation to be performed by users on images they
do not own, you can set the `copy_image` policy to something other
than the default, for example:

```
"copy_image": "'public':%(visibility)s"
```

For example, use the following configuration to enable `copy-imge` import method:

```
...
  glance:
    template:
      customServiceConfig: |
        [DEFAULT]
        enabled_import_methods=web-download,copy-image
      glanceAPIs:
        ...
...
...
```

Assuming you are using `install_yamls` and you already have `crc` running, you
can use the provided `copy_image` example with:

```
$ cd install_yamls
$ make ceph TIMEOUT=90
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/copy_image > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

If we already have a deployment working we can always use `oc kustomize ../copy_image | oc apply -f -`
from this directory to make the changes.

## Copying existing image to multiple stores

This feature enables you to copy existing images using Red Hat
OpenStack Image service (glance) image data into multiple Red Hat
Ceph Storage stores using the interoperable image import workflow.

```
The image must be present atleast in one store before you copy it
to any other store(s). Only the image owner or administrator can copy
existing images to newly added stores.
```

You can copy existing image data either by setting `--all-stores` to `true`
or by specifying specific stores to receive the image data.

- The default setting for the `--all-stores` option is `false`. If `--all-stores`
  is `false`, you must specify which stores receive the image data by using
  `--stores <store-1>,<store-2>`. If the image data is already present in any
  of the specified stores, the request fails.
- If you set `all-stores` to true, and the image data already exists in some
  of the stores, then those stores are excluded from the list.

Once the `OpenStackControlPlane` is ready, it is possible to copy an existing
image to a particular store, or to all available stores.
To list the available stores, run the following command:

```bash
$ glance --os-auth-url $keystone --os-project-name admin --os-username admin \
         --os-password $password --os-user-domain-name default \
         --os-project-domain-name default stores-info

+----------+----------------------------------------------------------------------------------+
| Property | Value                                                                            |
+----------+----------------------------------------------------------------------------------+
| stores   | [{"id": "ceph-0", "description": "RBD backend"}, {"id": "ceph-1", "description": |
|          | "RBD backend 1", "default": "true"}, {"id": "swift-0"}]                          |
+----------+----------------------------------------------------------------------------------+
```

To list the available image(s), run the following command:

```bash
$ glance --os-auth-url $keystone --os-project-name admin --os-username admin \
         --os-password $password --os-user-domain-name default \
         --os-project-domain-name default image-list --include-stores

+--------------------------------------+-------------------+--------+
| ID                                   | Name              | Stores |
+--------------------------------------+-------------------+--------+
| 6f0b6926-ea71-4da1-9d77-28074efd90e4 | cirros            | ceph-0 |
+--------------------------------------+-------------------+--------+
```

To copy an image on a particular store, for instance `swift-0`,
run the following command:

```bash
$ glance --os-auth-url $keystone --os-project-name admin --os-username admin \
         --os-password $password --os-user-domain-name default \
         image-import <image-id> --stores <store-1> --import-method copy-image
```

- Replace <image-id> with the `6f0b6926-ea71-4da1-9d77-28074efd90e4` of the image you want to copy.
- Replace <store-1> with name of the store `swift-0` to copy the image data.

To copy an image (e.g., cirros) on all the stores, run the following command:

```bash
$ glance --os-auth-url $keystone --os-project-name admin --os-username admin \
         --os-password $password --os-user-domain-name default \
         image-import <image-id> --all-stores true --import-method copy-image
```

- Replace <image-id> with the `6f0b6926-ea71-4da1-9d77-28074efd90e4` of the image you want to copy.

To verify image is now copied to all stores, run the following command:

```bash
$ glance --os-auth-url $keystone --os-project-name admin --os-username admin \
         --os-password $password --os-user-domain-name default \
         --os-project-domain-name default image-list --include-stores

+--------------------------------------+-------------------+-----------------------+
| ID                                   | Name              | Stores                |
+--------------------------------------+-------------------+-----------------------+
| 6f0b6926-ea71-4da1-9d77-28074efd90e4 | cirros            | ceph-0,swift-0,ceph-1 |
+--------------------------------------+-------------------+-----------------------+
```

You can find more about `copy-image` import method in the
[upstream](https://docs.openstack.org/glance/latest/admin/interoperable-image-import.html#configuring-the-copy-image-method) documentation.
