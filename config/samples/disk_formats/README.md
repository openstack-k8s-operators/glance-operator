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

## How to test 
The steps and overview about a feature described in [disk-format](../../../../config/samples/disk_formats/) document  
We assume one GlanceAPIs exist, disk format is enabled with disk formats
'raw, iso' and image is created with same disk format.

### Step 1:  Create image
In this step we create images with 'raw' and 'iso' disk formats
```bash
    openstack image create \
        --disk-format "$1" \
        --container-format bare \
        "${IMAGE_NAME}"
```

## EXAMPLE

The example assumes a glanceAPI is deployed using [single layout](https://github.com/openstack-k8s-operators/glance-operator/tree/main/config/samples/layout/single).

Copy the [`create-image.sh`](create-image.sh) script to the target container
where the `openstack` cli is available.
For example:

```bash
$oc cp create-image.sh openstackclient:/scripts
```

Create image with 'raw' disk format

```bash
sh-5.1# bash create-image.sh raw
openstack image create --disk-format raw --container-format bare myimage-disk_format-test
+------------------+--------------------------------------+
| Property         | Value                                |
+------------------+--------------------------------------+
| checksum         | None                                 |
| container_format | bare                                 |
| created_at       | 2024-05-02T13:15:45Z                 |
| disk_format      | raw                                  |
| id               | 32737bf5-2546-4d6f-9800-80cc5afed30d |
| locations        | []                                   |
| min_disk         | 0                                    |
| min_ram          | 0                                    |
| name             | myimage-disk_format-test             |
| os_hash_algo     | None                                 |
| os_hash_value    | None                                 |
| os_hidden        | False                                |
| owner            | 7ac7162bc58f44af824fd8f2ce68987f     |
| protected        | False                                |
| size             | None                                 |
| status           | queued                               |
| tags             | []                                   |
| updated_at       | 2024-05-02T13:15:45Z                 |
| virtual_size     | Not available                        |
| visibility       | shared                               |
+------------------+--------------------------------------+

+ openstack image list 
+--------------------------------------+--------------------------+
| ID                                   | Name                     |
+--------------------------------------+--------------------------+
| 32737bf5-2546-4d6f-9800-80cc5afed30d | myimage-disk_format-test |
+--------------------------------------+--------------------------+

+ echo 'Successfully created image'
Successfully created image
```
