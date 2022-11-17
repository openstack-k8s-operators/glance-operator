# glance-operator

A Kubernetes Operator built using the [Operator Framework for Go](https://github.com/operator-framework).
The Operator provides a way to install and manage the OpenStack Glance component
on Kubernetes.

## Deployment

The operator is intended to be deployed via [OLM Operator Lifecycle Manager](https://sdk.operatorframework.io/docs/olm-integration/quickstart-bundle).

For development purposes, it can be deployed using [install_yamls](https://github.com/openstack-k8s-operators/install_yamls) project.

```
# clone the install_yamls repository
git clone https://github.com/openstack-k8s-operators/install_yamls

# move to the install_yamls directory
cd install_yamls

# one time operation to initialize PVs within the CRC VM, required by glance to start
make crc_storage

# Install Glance Operator using OLM (defaults to quay.io/openstack-k8s-operators)
make glance GLANCE_IMG=quay.io/openstack-k8s-operators/glance-operator-index:latest

# Deploy Glance
make glance_deploy
```

## Example: configure Glance with Ceph backend

The Glance spec API can be used to configure and customize the Ceph backend. In
particular, the presence of the cephBackend data structure has the effect of
creating an override in the glance config, adding rbd as default backend
parameter. The global `cephBackend` parameter is used to specify the Ceph
client-related "key/value" pairs required to connect the service with an
external Ceph cluster. Multiple external Ceph clusters are not supported at the
moment. The following represents an example of Glance resource that can be used
to trigger the service deployment, and enable the rbd backend that points to an
external Ceph cluster.

```
apiVersion: glance.openstack.org/v1beta1
kind: GlanceAPI
metadata:
  name: glance
spec:
  serviceUser: glance
  containerImage: quay.io/tripleowallabycentos9/openstack-glance-api:current-tripleo
  customServiceConfig: |
    [DEFAULT]
    debug = true
  databaseInstance: openstack
  databaseUser: glance
  debug:
    dbSync: false
    service: false
  preserveJobs: false
  replicas: 1
  storageRequest: 1G
  secret: glance-secret
  cephBackend:
    cephFsid: <ClusterFSID>
    cephMons: <CephMons>
    cephClientKey: <ClientKey>
    cephUser: <rbdUser>
    cephPools:
      cinder:
        name: volumes
      nova:
        name: vms
      glance:
        name: images
      cinder_backup:
        name: backup
      extra_pool1:
        name: ceph_ssd_tier
      extra_pool2:
        name: ceph_nvme_tier
      extra_pool3:
        name: ceph_hdd_tier
```

When the service is up and running, it's possible to interact with the glance
API and upload an image using the Ceph backend.

## Example: Testing Glance policies

Glance's public API calls may be restricted to certain sets of users using a
policy configuration file. This section explains exactly how to test APIs with
policies. Glance operator configures default policies in policy.yaml file which
will be stored at '/etc/glance' section. The following represents how you can
test API(s) with glance policies.

```
Create below projects, users and assign member and reader roles to each of them;

openstack project create --description 'project a' project-a --domain default
openstack project create --description 'project b' project-b --domain default

openstack user create project-a-reader --password project-a-reader
openstack user create project-b-reader --password project-b-reader
openstack user create project-a-member --password project-a-member
openstack user create project-b-member --password project-b-member

openstack role add --user project-a-member --project project-a member
openstack role add --user project-a-reader --project project-a reader
openstack role add --user project-b-member --project project-b member
openstack role add --user project-b-reader --project project-b reader

Create project-a-reader-rc, project-a-member-rc, project-b-reader-rc, project-b-member-rc using below contents;

export OS_AUTH_URL=<auth-url>
export OS_PASSWORD='project-a-member'
export OS_PROJECT_DOMAIN_NAME=Default
export OS_PROJECT_NAME='project-a'
export OS_USER_DOMAIN_NAME=Default
export OS_USERNAME='project-a-member'
export OS_CACERT=/etc/pki/ca-trust/source/anchors/simpleca.crt
export OS_IDENTITY_API_VERSION=3
export OS_REGION_NAME=regionOne
export OS_VOLUME_API_VERSION=3

Note, don't forget to change the password, username and projectname for each rc file accordingly.

Now source project-a-member-rc file
$ source project-a-member-rc

1. Run glance image-create command to create private image
$ glance image-create --disk-format qcow2 --container-format bare --name cirros --file <file_path_of_image> --visibility private

Image will be created successfully and in active state.

Now source project-a-reader-rc file
$ source project-a-reader-rc file

2. Run glance image-create command again
$ glance image-create --disk-format qcow2 --container-format bare --name cirros --file <file_path_of_image>

Since reader role is not permitted to create/update/delete action, you will get 403 Forbidden response.
 
3. Run glance image-list command
$ glance image-list

You will be able to see image created in Step 1.

Now source project-b-reader-rc file
$ source project-b-reader-rc file

4. Run glance image-list command
$ glance image-list

You will not be able to see image created in Step 1 as it is private to project-a.
```

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

