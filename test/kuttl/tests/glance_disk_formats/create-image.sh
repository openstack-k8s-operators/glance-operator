#!/bin/bash
set -evx
# Create a dummy image with disk_format and container_format
#
# The scripts assumes:
#
# 1. an available glance cli
# 2. a single layout (file / NFS) backend and disk format is deployed
# 3. pass the password via environment variable, for example:
#
#    export PASSWORD=12345678
#
#
AUTH_URL=${AUTH_URL:-"https://keystone-public.openstack.svc:5000"}
USER=${USER:-"admin"}
TIME=5
DOMAIN=${DOMAIN:-"glance-default-single.openstack.svc:9292"}
IMAGE_NAME="myimage-test"
EXIT_CODE=$?


function create_image() {
    # This method is create, list and delete created image
    # $1 - disk format
    # $2 - container format
    glance="glance --os-auth-url ${AUTH_URL} \
        --os-project-name ${USER} \
        --os-username ${USER} \
        --os-password ${PASSWORD} \
        --os-user-domain-name default \
        --os-project-domain-name default "

    echo This is a dodgy image > "${IMAGE_NAME}"

    $glance --verbose image-create \
        --disk-format "$1" \
        --container-format "$2" \
        --name "${IMAGE_NAME}" 

    ID=$($glance image-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
    echo "Image ID: $ID"
    sleep "${TIME}"

    if [ -z "$ID" ]
    then
      echo "Could not create image  " >&2; exit 1
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

create_image "$1" "$2"
if [ -z $? ]
then
  echo "Could not create image  "; exit 1
elif [ $EXIT_CODE == 0 ]
then
  echo "Successfully created image"
else
  echo "Could not create image  " >&2; exit 1 
fi
