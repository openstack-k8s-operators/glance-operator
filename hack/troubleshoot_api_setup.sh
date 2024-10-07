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

# Setup glance config
function setup_glance {
    echo "Generate glance.conf.d"
    /usr/local/bin/kolla_set_configs
    echo "Run extend_start"
    /usr/local/bin/kolla_extend_start
    echo "Setup httpd"
    cp /var/lib/config-data/default/httpd.conf /etc/httpd/conf.d/
    cp /var/lib/config-data/default/ssl.conf /etc/httpd/conf.d/
    cp /var/lib/config-data/default/10-glance-httpd.conf /etc/httpd/conf.d/10-glance.conf
}

# copy files
setup_glance
# run glance-api
glance-api --config-dir "$CONFIG_DIR" &
/usr/sbin/httpd -DFOREGROUND &
# test the client run image-list
