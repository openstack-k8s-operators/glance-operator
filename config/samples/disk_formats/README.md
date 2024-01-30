# Supported disk-formats

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

You can find more about disk-formats configuration options in the
[upstream](https://docs.openstack.org/glance/latest/configuration/configuring.html#configuring-supported-disk-formats) documentation.
