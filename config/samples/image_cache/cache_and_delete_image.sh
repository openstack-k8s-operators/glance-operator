#!/bin/env bash
# Upload a dummy image and test cache related commands
#
# The scripts assumes:
#
# 1. an available glance cli
#
# 2. a single layout is deployed (file / NFS) backend
#
# 3. two glance-api replicas are used: in this case, make sure to patch the
#    existing osctlplane with the following command:
#
#    oc patch glance glance --type=json -p="[{'op': 'replace', 'path': '/spec/glanceAPIs/default/replicas', value: 2}]"
#
# 4. Retrieve the cloud credential from the existing 'clouds.yaml' with the
#    following command:
#
#    oc get cm openstack-config -o json | jq -r '.data["clouds.yaml"]'
#
# 5. pass the password via environment variable, for example:
#
#    export PASSWORD=12345678

set -x

TIME=6
CACHE_TIME=6
DOMAIN=${DOMAIN:-"glance-default-single.openstack.svc"}
REPLICA=${REPLICA:-"glance-default-single-"}
IMAGE_NAME="myimage"
KEYSTONE=$(awk '/auth_url/ {print $2}' "/etc/openstack/clouds.yaml")
ADMIN_PWD=${1:-12345678}
ADMIN_USER=${ADMIN_USER:-"admin"}

# this method uses distributed image import and relies on the glance cli
glance="glance --os-auth-url ${KEYSTONE} \
    --os-project-name ${ADMIN_USER} \
    --os-username ${ADMIN_USER} \
    --os-password ${ADMIN_PWD} \
    --os-user-domain-name default \
    --os-project-domain-name default "
# disable stdin
exec 0<&-

# Build a dodgy image
echo This is a dodgy image > $HOME/"${IMAGE_NAME}"

# Stage 0 - Delete any pre-existing image
openstack image list -c ID -f value | xargs -n 1 openstack image delete

# Stage 1 - Verify no image is cached on replica 0 and replica 1
CACHED_ID=$($glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-list | awk -v state=cached '$0 ~ state {print $2}')
if [[ $CACHED_ID != "" ]]; then
    echo "Image is already cached on replica 0, exiting!"
    exit 1
fi

CACHED_ID=$($glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-list | awk -v state=cached '$0 ~ state {print $2}')
if [[ $CACHED_ID != "" ]]; then
    echo "Image is already cached on replica 1, exiting!"
    exit 1
fi

# Stage 2 - Create an image
echo "Creating new image."
$glance --verbose image-create \
    --disk-format qcow2 \
    --container-format bare \
    --name "${IMAGE_NAME}" \
    --file myimage
sleep "$TIME"
ID=$($glance image-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
echo "Image ID: $ID"
sleep "$TIME"

# check the resulting image is in active state
STATE=$($glance image-show "$ID" | awk '/status/{print $4}')
echo "Image Status => $STATE"
if [[ $STATE != "active" ]]; then
    echo "Image is not in active state, exiting!"
    exit 1
fi

# Stage 2 - Cache an image on replica 0
echo "Caching image on replica 0"
$glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-queue "$ID"
sleep "$CACHE_TIME"

# Stage 3 - Verify that image is cached on replica 0 and not on replica 1
CACHED_ID=$($glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-list | awk -v state=cached '$0 ~ state {print $2}')
echo "Cached image id on replica 0 => $CACHED_ID"
if [[ $CACHED_ID != $ID ]]; then
    echo "Failed to cache image on replica 0, exiting!"
    exit 1
fi

echo "Verifying image is not cached on replica 1"
CACHED_ID_1=$($glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-list | awk -v state=cached '$0 ~ state {print $2}')
echo "Cached image id on replica 1 => $CACHED_ID_1"
if [[ $CACHED_ID_1 != "" ]]; then
    echo "Image expected to cached on replica 0 and not on replica 1, exiting!"
    exit 1
fi

# Stage 4 - Cache an image on replica 1
echo "Caching image on replica 1"
$glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-queue "$ID"
sleep "$CACHE_TIME"

# Stage 5 - Verify that image is cached on replica 1
CACHED_ID_2=$($glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-list | awk -v state=cached '$0 ~ state {print $2}')
echo "Cached image id on replica 1 => $CACHED_ID_2"
if [[ $CACHED_ID_2 != $ID ]]; then
    echo "Failed to cache image on replica 1, exiting!"
    exit 1
fi

# Stage 6 - Delete the cached image from replica 0
echo "Deleting cached image from replica 0"
$glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-delete "$CACHED_ID"

# Stage 7 - Verify that image is still cached on replica 1 and deleted from replica 0
echo "Verifying image is still cached on replica 1"
CACHED_ID_3=$($glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-list | awk -v state=cached '$0 ~ state {print $2}')
echo "Cached image id on replica 1 => $CACHED_ID_3"
if [[ $CACHED_ID_3 != $ID ]]; then
    exit 1
fi

echo "Verify Cached image is deleted from replica 0"
CACHED_ID_4=$($glance --os-image-url "http://${REPLICA}""0.$DOMAIN:9292" cache-list | awk -v state=cached '$0 ~ state {print $2}')
if [[ $CACHED_ID_4 != "" ]]; then
    echo "Cached image $CACHED_ID_4 is not deleted from replica 0"
    exit 1
fi

# Stage 8 - Delete the actual image
echo "Deleting image $ID"
$glance image-delete "$ID"


# Stage 9 - Delete cached image from replica 1 if it is still present
CACHED_ID_5=$($glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-list | awk -v state=cached '$0 ~ state {print $2}')
if [[ $CACHED_ID_5 != "" ]]; then
    echo "Deleting cached image from replica 1"
    $glance --os-image-url "http://${REPLICA}""1.$DOMAIN:9292" cache-delete $CACHED_ID_5
fi

echo "Caching tests executed successfully!!!"
exit 0
