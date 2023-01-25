#!/bin//bash
#
# Copyright 2020 Red Hat Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.
set -ex

# This script generates the glance-api.conf/logging.conf file and
# copies the result to the ephemeral /var/lib/config-data/merged volume.
#
# Secrets are obtained from ENV variables.
export DBPASSWORD=${DatabasePassword:?"Please specify a DatabasePassword variable."}
# TODO
#export TRANSPORTURL=${TransportUrl:?"Please specify a TransportUrl variable."}
export GLANCEPASSWORD=${GlancePassword:?"Please specify a GlanceKeystoneAuthPassword variable."}
export DBHOST=${DatabaseHost:?"Please specify a DatabaseHost variable."}
export DBUSER=${DatabaseUser:-"glance"}
export DB=${DatabaseName:-"glance"}

SVC_CFG=/etc/glance/glance-api.conf
SVC_CFG_MERGED=/var/lib/config-data/merged/glance-api.conf

# expect that the common.sh is in the same dir as the calling script
SCRIPTPATH="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
. ${SCRIPTPATH}/common.sh --source-only

# Copy default service config from container image as base
cp -a ${SVC_CFG} ${SVC_CFG_MERGED}

# Merge all templates from config-data
for dir in /var/lib/config-data/default
do
  merge_config_dir ${dir}
done

# set secrets
# TODO: transportUrl (either set here or elsewhere)
#crudini --set ${SVC_CFG_MERGED} DEFAULT transport_url $TRANSPORTURL
crudini --set ${SVC_CFG_MERGED} database connection mysql+pymysql://${DBUSER}:${DBPASSWORD}@${DBHOST}/${DB}
crudini --set ${SVC_CFG_MERGED} keystone_authtoken password $GLANCEPASSWORD
# TODO: transportUrl (either set here or elsewhere)
#crudini --set ${SVC_CFG_MERGED} oslo_messaging_notifications transport_url $TRANSPORTURL
