#!/bin/env bash
#
#  Get the cloud config via the following command:
#
#  $ oc get cm openstack-config -o json | jq -r '.data["clouds.yaml"]'
#
#  Note: pass AUTH_URL, USER and PASSWORD via environment variable, for example:
#
#  $ oc get secret/openstack-config-secret -o json | jq -r '.data["secure.yaml"]' | base64 -d
#
#  $ export AUTH_URL="http://keystone-public.openstack.svc:5000/v3"
#  $ export USER=admin
#  $ export PASSWORD=12345678
#
#  $ export glance="glance --os-auth-url ${AUTH_URL} \
#    --os-project-name ${USER} \
#    --os-username ${USER} \
#    --os-password ${PASSWORD} \
#    --os-user-domain-name default \
#    --os-project-domain-name default \
#    --os-image-url http://localhost:9292 "

CONFIG_DIR=${CONFIG_DIR:-/etc/glance/glance.conf.d}

# Config files (00/02/03-config.conf, my.cnf, httpd.conf, the wsgi/proxypass
# vhost, ssl.conf) are already mounted directly at their final locations --
# no kolla-style copy step is needed anymore. If GLANCE_DOMAIN is set (only
# relevant for distributed image import), write the runtime-only
# worker_self_reference_url snippet the container command would normally
# generate on startup:
if [ -n "$GLANCE_DOMAIN" ]; then
    cat > "$CONFIG_DIR/01-config.conf" <<EOF
[DEFAULT]
worker_self_reference_url=${URISCHEME,,}://$(hostname).${GLANCE_DOMAIN}:${GLANCE_PORT}
EOF
fi

# run glance-api
glance-api --config-dir "$CONFIG_DIR" &
/usr/sbin/httpd -DFOREGROUND &
# test the client run image-list
