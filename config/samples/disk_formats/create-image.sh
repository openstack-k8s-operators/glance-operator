#!/bin/bash
set -evx
# Create a image with supported disk format
#
# The scripts assumes:
#
# 1. an available openstack cli
# 2. control plane configured with disk formats
# 2. a single layout (file / NFS) backend and disk format is deployed
#
#
TIME=5
IMAGE_NAME="myimage-disk_format-test"
EXIT_CODE=$?


function create_image() {
    # This method is create, list and delete image
    # $1 - disk format

    echo This is a dodgy image > "${IMAGE_NAME}"

    openstack image create \
        --disk-format "$1" \
        --container-format bare \
        "${IMAGE_NAME}" 

    ID=$(openstack image list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
    echo "Image ID: $ID"
    sleep "${TIME}"

    if [ -z "$ID" ]
    then
      echo "Could not create image  " >&2
      openstack image delete "$ID"
      status=$(openstack image delete "$ID" | awk '/status/{print $4}')
      printf "Image Status: %s\n" "$status"
      exit 1
    else
      echo "Continue"
    fi

    # Stage 2 - Check the image is active
    openstack image list
    status=$(openstack image show "$ID" | awk '/status/{print $4}')
    printf "Image Status: %s\n" "$status"

    # Stage 3 - Delete the image 
    openstack image delete "$ID"
    status=$(openstack image delete "$ID" | awk '/status/{print $4}')
    printf "Image Status: %s\n" "$status"
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
