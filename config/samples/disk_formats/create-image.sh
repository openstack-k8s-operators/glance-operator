#!/bin/bash
set -evx
# Create a image with supported disk format
#
# The scripts assumes:
#
# 1. an available openstack cli
# 2. glanceAPI configured with disk formats
# 3. a single layout (file / NFS) backend and disk format is deployed
#
#
TIME=5
IMAGE_NAME="myimage-disk_format-test"
EXIT_CODE=$?
DEBUG=0


function create_image() {
    # This method is create, list and delete image
    # $1 - disk format
    # $2 - container format

    echo This is a dodgy image > "${IMAGE_NAME}"

    # Stage 0 - Delete any pre-existing image
    openstack image list -c ID -f value | xargs -n 1 openstack image delete
    sleep "${TIME}"

    # Stage 1 - Create image
    openstack image create \
        --disk-format "$1" \
        --container-format bare \
        "${IMAGE_NAME}"

    ID=$(openstack image list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
    echo "Image ID: $ID"
    sleep "${TIME}"

    if [ -z "$ID" ]
    then
        openstack image list -c ID -f value | xargs -n 1 openstack image delete
        exit 1
    fi

    # Stage 2 - Check the image is active
    if [ "$DEBUG" -eq 1 ]; then
        openstack image list

    status=$(openstack image show "$ID" | awk '/status/{print $4}')
    if [ "$status" == 'active' ]
    then
        printf "Image Status: %s\n" "$status"
        exit 0
    else
        printf "Image Status: %s\n" "$status"
        exit 1
    fi

}

create_image "$1"
if [ -z $? ]
then
    echo "Could not create image  "; exit 1
elif [ $EXIT_CODE == 0 ]
then
    echo "Successfully created image"
else
    echo "Could not create image  " >&2; exit 1
fi
