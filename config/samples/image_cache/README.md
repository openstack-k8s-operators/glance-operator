# Glance image-cache

The Glance API server may be configured to have an optional local image cache.
A local image cache stores a copy of image files, enabling multiple API servers
to serve the same image file in a more scalable fashion due to an increased
number of endpoints serving an image file.
The local image cache is transparent to the end user: the end user does not know
that the Glance API is streaming an image file from its local cache or from the
actual backend storage system.
To enable the `Glance` image-cache feature, the human operator must specify in
the main Glance CR the size assigned to the Cache by adding the `Size` parameter
to the `ImageCache` section:


```
...
  glance:
    template:
      customServiceConfig: |
        ...
        ...
        ...
        ...
      databaseInstance: openstack
      imageCache:
        size: 10Gi
      storage:
        storageRequest: 10Gi
        storageClass: ""
      glanceAPIs:
        default:
        ...
...
...
```

The format is the consistent with the [Kubernetes way of defining PVCs claims](https://kubernetes.io/docs/concepts/storage/persistent-volumes/),
which is used to define a regular `storageRequest` when Glance is deployed.

## Enable image-cache with a Ceph backend

Assuming you are using `install_yamls` and you already have `crc` running, you
can use the provided `image_cache` example with:

```
$ cd install_yamls
$ make ceph TIMEOUT=90
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/image_cache > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

If we already have a deployment working we can always use `oc kustomize ../image_cache | oc apply -f -`
from this directory to make the changes.

## Glance Cache cleaner and pruner utilities

When images are successfully returned from a call to GET /images/<IMAGE_ID>,
the image cache automatically writes the image file to its cache, regardless of
whether the resulting write would make the image cache’s size exceed the value
of `image_cache_max_size`, which is defined by the `size` parameter exposed in
the `imageCache` section of the `Glance` CR. In order to keep the image cache
at or below this maximum cache size, Glance provides utilities that can be
periodically executed.
The glance-operator defines a `cronJob` resource that periodically executes the
`glance-cache-pruner` utility, with the purpose of keeping under the
`image_cache_max_size` value the image cache size.
Over time, the image cache can accumulate image files that are either in a
stalled or invalid state. Stalled image files are the result of an image cache
write failing to complete. Invalid image files are the result of an image file
not being written properly to disk.
The amount of time an incomplete image can stay in the cache is defined by the
`image_cache_stall_time` parameter (which defaults to 86400 seconds), after
this time the incomplete or stalled image is qualified to be deleted from the
`Glance` cache.
To remove these types of files, the `glance-operator` defines a `cronJob`
resource that periodically executes the `glance-cache-cleaner` utility.

You can find more about image-cache configuration options in the
[upstream](https://docs.openstack.org/glance/latest/admin/cache.html) documentation.

## How to test

Assuming a given `GlanceAPI` instance has been scaled up, it is possible to
perform cache related operations and verify that `Pods` are able to see each
other (or resolve them by `hostname`) and the image can be cached on either
of the replicas or deleted from either of the replicas.

As glance cache are local, each glance `Pod` can cache the image locally
using `--os-image-url` as shown below in Step 2. If you have multiple glance
pods running and using `file` backend for storing the image then you need
to ensure that your `filepath` is mounted using `NFS` otherwise caching
will not work as expected.

### Step 1: Create an image

The first step is to create an image to be cached at later step.

```bash
  glance --verbose image-create \
   --disk-format qcow2 \
   --container-format bare \
   --name $IMAGE_NAME
   --file myimage
```

### Step 2: Cache an image on replica 0

```bash
UUID=$(openstack image show $IMAGE_NAME -c id -f value)
glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-queue "$UUID"
```

- **REPLICA**: a glanceAPI replicas that doesn’t own the data

- **DOMAIN**: Service hostname associated with a particular GlanceAPI

  - These commands assume we have minimum 2 REPLICAS available

## Step 3: Verify that image is cached on replica 0 and not on replica 1

```bash
glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
# This should return 1 image in output
glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
# This should return 0 image in output
```
- **IMAGE_NAME**: the name of the image box that will be backed by the imported
  data

- **REPLICA**: a glanceAPI replicas that doesn’t own the data

- **DOMAIN**: Service hostname associated with a particular GlanceAPI

- These commands assume we have minimum 2 REPLICAS available

## Step 4: Cache an image on replica 1
```bash
UUID=$(openstack image show $IMAGE_NAME -c id -f value)
glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-queue "$UUID"
```

- **REPLICA**: a glanceAPI replicas that doesn’t own the data

- **DOMAIN**: Service hostname associated with a particular GlanceAPI

- These commands assume we have minimum 2 REPLICAS available

## Step 5: Verify that image is cached on replica 1

```bash
glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
# This should return 0 image in output
```
- **IMAGE_NAME**: the name of the image box that will be backed by the imported
  data

- **REPLICA**: a glanceAPI replicas that doesn’t own the data

- **DOMAIN**: Service hostname associated with a particular GlanceAPI

- These commands assume we have minimum 2 REPLICAS available

## Step 6: Delete the cached image from replica 0
```bash
UUID=$(openstack image show $IMAGE_NAME -c id -f value)
glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-delete "$UUID"
```

## Step 7: Verify that image is still cached on replica 1 and deleted from replica 0
```bash
glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
# This should return 0 image in output
glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
# This should return 1 image in output
```
- **IMAGE_NAME**: the name of the image box that will be backed by the imported
  data

- **REPLICA**: a glanceAPI replicas that doesn’t own the data

- **DOMAIN**: Service hostname associated with a particular GlanceAPI

- These commands assume we have minimum 2 REPLICAS available

## Step 8: Delete the actual image and verify that it is deleted from cached images as well
```bash
UUID=$(openstack image show $IMAGE_NAME -c id -f value)
glance image-delete "$UUID"
glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
# This should return 0 image in output
glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
# This should return 0 image in output
```
- **IMAGE_NAME**: the name of the image box that will be backed by the imported
  data

- **REPLICA**: a glanceAPI replicas that doesn’t own the data

- **DOMAIN**: Service hostname associated with a particular GlanceAPI

- These commands assume we have minimum 2 REPLICAS available
