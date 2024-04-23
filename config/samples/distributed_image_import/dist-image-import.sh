#!/bin/env bash
# Upload a dummy image using distributed image import
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

TIME=3
DOMAIN=${DOMAIN:-"glance-default-single.openstack.svc:9292"}
REPLICA="glance-default-single-"
IMAGE_NAME="myimage"

keystone=$(awk '/auth_url/ {print $2}' "/etc/openstack/clouds.yaml")
admin_pwd=${1:-12345678}
admin_user=${USER:-"admin"}

# this method uses distributed image import and relies on the glance cli
glance="glance --os-auth-url ${keystone} \
    --os-project-name ${admin_user} \
    --os-username ${admin_user} \
    --os-password ${admin_pwd} \
    --os-user-domain-name default \
    --os-project-domain-name default "
# disable stdin
exec 0<&-

# Build a dodgy image
echo This is a dodgy image > "${IMAGE_NAME}"

# Stage 0 - Delete any pre-existing image
openstack image list -c ID -f value | xargs -n 1 openstack image delete

# Stage 1 - Create an empty box
$glance --verbose image-create \
    --disk-format qcow2 \
    --container-format bare \
    --name "${IMAGE_NAME}"
ID=$($glance image-list | awk -v img=$IMAGE_NAME '$0 ~ img {print $2}')
echo "Image ID: $ID"

# check the resulting image is in queued state
STATE=$($glance image-show "$ID" | awk '/status/{print $4}')
echo "Image Status => $STATE"
sleep "$TIME"

# Stage 2 - Stage the image
echo "$glance image-stage --progress --file myimage $ID"
$glance --os-image-url "http://${REPLICA}""0.$DOMAIN" image-stage --progress --file "${IMAGE_NAME}" "$ID"

# Stage 3 - Import the image from a different replica
echo "$glance image-import --progress --file ${IMAGE_NAME} $ID"
$glance --os-image-url "http://${REPLICA}""1.$DOMAIN" image-import --import-method glance-direct "$ID"

# Stage 4 - Check the image is active
$glance image-list
status=$($glance image-show "$ID" | awk '/status/{print $4}')
printf "Image Status: %s\n" "$status"
