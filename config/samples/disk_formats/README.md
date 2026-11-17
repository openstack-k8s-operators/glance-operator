# Supported disk formats and legacy upload restrictions

Each image in Glance has an associated disk format property.
When creating an image the user specifies a disk format. They must
select a format from the set that the Glance service supports.
An operator can add or remove disk formats to the supported set.  This is
done by setting the ``disk_formats`` parameter which is found in the
``[image_format]`` section of ``glance-api.conf``.

``disk_formats=<Comma separated list of disk formats>``

Default supported formats are: ``ami,ari,aki,vhd,vhdx,vmdk,raw,qcow2,vdi,iso,ploop``

For example, use the following configuration to enable only RAW and ISO disk formats:

```
...
  glance:
    template:
      customServiceConfig: |
        [image_format]
        disk_formats=raw,iso
      glanceAPIs:
        ...
...
...
```

For example, use the following configuration to reject QCOW2 disk images:

```
...
  glance:
    template:
      customServiceConfig: |
        [image_format]
        disk_formats=raw,iso,aki,ari,ami
      glanceAPIs:
        ...
...
...
```

## Restricting legacy uploads to specific formats

``disk_formats`` controls which formats a user may declare when creating or
updating an image, but it does not control how the image content is uploaded.
Glance has two upload paths:

- **Interoperable image import** (``glance-direct``, ``web-download``,
  ``copy-image``). The import workflow runs the configured
  ``image_import_plugins``, which may include ``image_conversion``.
- **Legacy (traditional) upload**, i.e.
  ``PUT /v2/images/{id}/file``. The bytes are written as they are received;
  the ``image_conversion`` plugin does **not** run on this path.

To restrict the legacy path by disk format, set ``allowed_upload_formats`` in
the same ``[image_format]`` section:

``allowed_upload_formats=<Comma separated list of disk formats>``

The default is empty, which applies no extra restriction and preserves the
existing behavior. When the list is non-empty, a
``PUT /v2/images/{id}/file`` request for an image whose ``disk_format`` is not
in the list is rejected with ``400 Bad Request`` before any data is stored.
The error tells the user to use the interoperable image import process, or one
of the allowed formats, instead.

This setting is independent from ``disk_formats``: it does not affect image
creation or the interoperable import process. It also does not replace the
``upload_image`` policy, which controls *who* may call the legacy upload API;
``allowed_upload_formats`` controls *which formats* may use it.

The operator does not set this option by default: it is opt-in and must be
added to ``customServiceConfig``.

**Version requirement:** this option requires a Glance image that includes the
``allowed-upload-formats`` feature (upstream 2027.1 and later). Older releases
silently ignore the unknown option, so the restriction is not enforced there.

### Example: raw-only storage with image conversion

```
[image_format]
disk_formats = raw,qcow2
allowed_upload_formats = raw
```

``qcow2`` stays in ``disk_formats`` so users can create qcow2 images and import
them; when image conversion is enabled (``output_format = raw``) the import
stores raw data and updates the image ``disk_format`` to ``raw``. A direct
qcow2 upload is rejected with ``400 Bad Request``, preventing the legacy path
from bypassing conversion. This is the recommended combination when the
storage backend should only ever hold raw images.

With the split layout on a Ceph RBD backend, the operator enables the
``image_conversion`` plugin with ``output_format = raw`` on the external API by
default, so the example above only needs ``allowed_upload_formats`` to complete
the picture.

### Example: qcow2-preferred backend

```
[image_format]
disk_formats = qcow2,raw,vmdk
allowed_upload_formats = qcow2
```

Raw imports are converted to qcow2; direct raw and vmdk uploads are rejected.
``vmdk`` is still accepted at image creation time for import purposes; remove
it from ``disk_formats`` if the backend does not support it.

### Example: allow qcow2 and vmdk uploads while converting imports to qcow2

```
[image_format]
disk_formats = qcow2,vmdk
allowed_upload_formats = qcow2,vmdk
```

Imported images are converted to the configured ``output_format``, while both
formats may also be uploaded directly through the legacy endpoint.

### Where to set the option

With the operator, add the section to ``Glance.spec.customServiceConfig`` (or
to ``Glance.spec.glanceAPIs.<name>.customServiceConfig`` for a per-API
override). The top-level value is inherited by every ``GlanceAPI`` that does
not define its own ``customServiceConfig``, so a single setting covers all API
instances. In a split layout only the external API receives user uploads, and
that is also the API where image conversion is enabled by default.

## Configuring supported disk-formats

Assuming you are using `install_yamls` and you already have `crc` running, you
can use the provided `disk_formats` example with:

```
$ cd install_yamls
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/disk_formats > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

If we already have a deployment working we can always use `oc kustomize ../disk_formats | oc apply -f -`
from this directory to make the changes.

You can find more about disk-formats and upload restrictions in the
[upstream](https://docs.openstack.org/glance/latest/configuration/configuring.html#configuring-supported-disk-formats) documentation.
