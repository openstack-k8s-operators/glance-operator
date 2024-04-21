#!/bin/bash
set -evx
# Create a image with supported disk format
#
# The scripts assumes:
#
# 1. an available glance cli
# 2. control plane configured with disk formats
# 2. a single layout (file / NFS) backend and disk format is deployed
#
#
KEYSTONE=$(awk '/auth_url/ {print $2}' "/$HOME/.config/openstack/clouds.yaml")
USER=${USER:-"admin"}
TIME=5
DOMAIN=${DOMAIN:-"glance-default-single.openstack.svc:9292"}
IMAGE_NAME="myimage-disk_format-test"
ADMIN_PWD=${ADMIN_PWD:-12345678}
EXIT_CODE=$?


function create_image() {
    # This method is create, list and delete image
    # $1 - disk format
    glance="glance --os-auth-url ${KEYSTONE}
        --os-project-name ${USER} \
        --os-username ${USER} \
        --os-password ${ADMIN_PWD} \
        --os-user-domain-name default \
        --os-project-domain-name default "

    echo This is a dodgy image > "${IMAGE_NAME}"

    $glance --verbose image-create \
        --disk-format "$1" \
        --container-format bare \
        --name "${IMAGE_NAME}" 

    ID=$($glance image-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
    echo "Image ID: $ID"
    sleep "${TIME}"

    if [ -z "$ID" ]
    then
      echo "Could not create image  " >&2
      $glance image-delete "$ID"
      status=$($glance image-delete "$ID" | awk '/status/{print $4}')
      printf "Image Status: %s\n" "$status"
      exit 1
    else
      echo "Continue"
    fi

    # Stage 2 - Check the image is active
    $glance image-list
    status=$($glance image-show "$ID" | awk '/status/{print $4}')
    printf "Image Status: %s\n" "$status"

    # Stage 3 - Delete the image 
    $glance image-delete "$ID"
    status=$($glance image-delete "$ID" | awk '/status/{print $4}')
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
