# Distributed Image Import kuttl test

This goal of this test is to verify that it is possible to proxy the image
upload request to the replica that has the data in its staging area. It assumes
that `glance-direct` is enabled, and the backend used for this verification is
not relevant. For this reason, the `glance-single` layout is deployed and we
can avoid the extra complexity brought by the `split` layout. The openstack cli
is used to perform actions against the glance instance resolved by keystone
catalog, hence only external would have been affected by this test.

## Kuttl test steps

The steps are mostly described with an example in the
[dist-image-import](../../../../config/samples/distributed_image_import/README.md)
document that gives an overview of the feature.
We assume two `GlanceAPIs` exist, and the steps perform a `pre-staged` upload
to verify that `Pods` are able to see each other and the request is properly
`proxied`.

### Step 1: Create an empty box

The first step is to create an empty box that will contain the image once is
uploaded through the import command.

```bash
  glance --verbose image-create \
   --disk-format qcow2 \
   --container-format bare \
   --name $IMAGE_NAME
```

- **IMAGE_NAME**: the name of the image that is going to be imported through
  the glance-direct method

### Step 2: Stage the image

```bash
UUID=$(openstack image show $IMAGE_NAME -c id -f value)
glance image-stage --progress --file $IMAGE_FILE $UUID
```

- **IMAGE_FILE**: the path of the image file that should be staged on a
  glanceAPI replica

## Step 3: Import the image through the glance-direct method

Assuming the staging operation happened on `replica-0`, which owns the data in
its staging directory (`/var/lib/glance/os_staging_store`), the idea is to
perform the import operation using a `$replica` that has no data in its
`os_staging_store` directory: we can verify how the request is properly proxied
to the node/pod/replica that owns the data.

```bash
UUID=$(openstack image show $IMAGE_NAME -c id -f value)
glance --os-image-url $replica image-import --import-method glance-direct $UUID
```
- **IMAGE_NAME**: the name of the image box that will be backed by the imported
  data

- **REPLICA**: a glanceAPI replicas that doesn’t own the data

## Conclusion

The steps described above are automated by this
[script](../../../../config/samples/distributed_image_import/dist-image-import.sh)
that is executed by the kuttl test once the environment is deployed and the
`openstackclient` is ready.
