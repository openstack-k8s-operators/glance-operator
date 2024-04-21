# How to test 

The goal of this test is to verify if user can set disk format for image that should be created

## Kuttl test steps
The steps and overview about a feature described in [disk-formats](../../../../config/samples/layout/disk_formats/) document
We assume one GlanceAPIs exist, disk format is enabled with disk formats
'raw, iso' and image is created with same disk format.

### Step 1:  Create image
In this step we create images with disk formats  'raw,iso'
```bash
    $openstack image create \
        --disk-format "$1" \
        --container-format $2 \
        "${IMAGE_NAME}"
```

## Conclusion
The steps described above are automated by this
[script](../../../../config/samples/disk_formats/create-image.sh)
that is executed by the kuttl test once the environment is deployed and the
`openstackclient` is ready.
