#!/usr/bin/env bash
#
# This is based on: https://docs.openstack.org/glance/latest/user/signature.html
# and it must be executed from the openstackClient POD

openssl genrsa -out private_key.pem 1024
openssl rsa -pubout -in private_key.pem -out public_key.pem
openssl req -new -key private_key.pem -out cert_request.csr
openssl x509 -req -days 14 -in cert_request.csr -signkey private_key.pem -out new_cert.crt

# create the secret and get the cert_uuid
cert_uuid=$(openstack secret store --name test --algorithm RSA --secret-type certificate --payload-content-type "application/octet-stream" --payload-content-encoding base64 --payload "$(base64 new_cert.crt)" -c "Secret href" -f value | cut -d '/' -f 6)

function build_image_signature {
    echo This is a dodgy image > myimage
    openssl dgst -sha512 -sign private_key.pem -sigopt rsa_padding_mode:pss -out myimage.signature myimage
    base64 -w 0 myimage.signature > myimage.signature.b64
}

function create_signed_image {
    local image_signature="$1"
    local cert_uuid="$2"
    local admin_pwd="$3"
    local keystone
    keystone=$(awk '/auth_url/ {print $2}' "$HOME"/.config/openstack/clouds.yaml)
    glance --os-auth-url "$keystone" \
        --os-project-name admin --os-username admin --os-password "$admin_pwd" \
        --os-user-domain-name default --os-project-domain-name default \
        image-create --name mySignedImage --container-format bare --disk-format qcow2 \
        --property img_signature="$image_signature" --property img_signature_certificate_uuid="$cert_uuid" \
        --property img_signature_hash_method='SHA-512' --property img_signature_key_type='RSA-PSS' < myimage
}

admin_pwd=${1:-12345678}
build_image_signature
image_signature=$(cat myimage.signature.b64)
create_signed_image "$image_signature" "$cert_uuid" "$admin_pwd"
