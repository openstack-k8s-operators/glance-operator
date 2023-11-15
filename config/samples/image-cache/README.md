# Glance image-cache

The Glance API server may be configured to have an optional local image cache.
A local image cache stores a copy of image files, enabling multiple API servers
to serve the same image file in a more scalable fashion due to an increased
number of endpoints serving an image file.
The local image cache is transparent to the end user: the end user does not know
that the Glance API is streaming an image file from its local cache or from the
actual backend storage system.
To enable the `Glance` image-cache feature, the human operator must specify in
the main Glance CR the size assigned to the Cache by adding the `imageCacheSize`
parameter:


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
      storageClass: ""
      imageCacheSize: 10G
      storageRequest: 10G
      glanceAPI:
        ...
...
...
```

The format is the consistent with the [Kubernetes way of defining PVCs claims](https://kubernetes.io/docs/concepts/storage/persistent-volumes/),
which is used to define a regular `storageRequest` when Glance is deployed.

## Enable image-cache with a Ceph backend

Assuming you are using `install_yamls` and you already have `crc` running, you
can use the provided `image-cache` example with:

```
$ cd install_yamls
$ make ceph TIMEOUT=90
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/image-cache > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

If we already have a deployment working we can always use `oc kustomize ../image-cache | oc apply -f -`
from this directory to make the changes.

## Glance Cache cleaner and pruner utilities

When images are successfully returned from a call to GET /images/<IMAGE_ID>, the
image cache automatically writes the image file to its cache, regardless of
whether the resulting write would make the image cache’s size exceed the value
of `image_cache_max_size`, which is defined by the `imageCacheSize` parameter
exposed by the top level `Glance` CR. In order to keep the image cache at or
below this maximum cache size, Glance provides utilities that can be periodically
executed.
The glance-operator defines a `cronJob` Pod that periodically executes the
`glance-cache-pruner` utility, with the purpose of keeping under the
`image_cache_max_size` value the image cache size.
Over time, the image cache can accumulate image files that are either in a
stalled or invalid state. Stalled image files are the result of an image cache
write failing to complete. Invalid image files are the result of an image file
not being written properly to disk.
To remove these types of files, the `glance-operator` defines a `cronJob` Pod
that periodically executes the `glance-cache-cleaner` utility.

You can find more about image-cache configuration options in the
[upstream](https://docs.openstack.org/glance/latest/admin/cache.html) documentation.
