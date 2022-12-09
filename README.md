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

## Launch glance-api-* service/process in debug mode 

Sometimes as a developer we need to make changes in configuration/policy files
or add more logs in actual code to see what went wrong in API calls. In normal
deployment it is not possible to do this on the fly and you need to redeploy
everything each time you make changes to either of above. If you launch the
glance-api container(pod) in debug mode then you will be able to do it on
the fly without relaunching/recreating the container each time. The following
represents how you can launch the glance pod in debug mode and make changes
on the fly.

```
Enable debug = True for the container you want to launch in debug mode, here
we want to launch userfacing container (External) in debug mode.

(path to file: install_yamls/out/openstack/glance/cr/glance_v1beta1_glance.yaml)

glanceAPIExternal:
    debug:
      service: true # Change it to true if it is false
    preserveJobs: false
    replicas: 1

Now rebuild the operator;

make generate && make manifests && make build
OPERATOR_TEMPLATES=$PWD/templates ./bin/manager

Once deployment is complete, you need to login to container;

oc exec -it glance-external-api-* bash

Verify that glance process is not running here;
$ ps aux | grep glance
root     1635263  0.0  0.0   6392  2296 pts/3    S+   07:35   0:00 grep --color=auto glance

Now you can modify the code to add pdb or more logs by doing actual
changes to code base;

$ python3 -c "import glance;print(glance.__file__)"
/usr/lib/python3.9/site-packages/glance/__init__.py

Add debug/log statements or pdb at your desired location in
/usr/lib/python3.9/site-packages/<file_name>.py

Launch the glance service;
/usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start

Verify that glance process is running;
$ ps aux | grep glance
root       13555  0.4  0.7 708732 123860 pts/1   S+   Nov15  35:33 /usr/bin/python3 /usr/bin/glance-api --config-dir /etc/glance/glance.conf.d
root       13590  0.0  0.6 711036 99096 pts/1    S+   Nov15   0:03 /usr/bin/python3 /usr/bin/glance-api --config-dir /etc/glance/glance.conf.d
root       13591  0.0  0.8 990744 135064 pts/1   S+   Nov15   0:04 /usr/bin/python3 /usr/bin/glance-api --config-dir /etc/glance/glance.conf.d
root       13592  0.0  0.8 990232 134864 pts/1   S+   Nov15   0:03 /usr/bin/python3 /usr/bin/glance-api --config-dir /etc/glance/glance.conf.d
root     1635263  0.0  0.0   6392  2296 pts/3    S+   07:35   0:00 grep --color=auto glance
 
Similar way you can modify configuration/policy files located in /etc/glance/* and kill
and start the service inside container.  
```

## Example: configure Glance with Ceph backend

The Glance spec can be used to configure Glance to connect to a Ceph
RBD server.

Create a secret which contains the Cephx key and Ceph configuration
file so that the Glance pod created by the operator can mount those
files in `/etc/ceph`.

```
---
apiVersion: v1
kind: Secret
metadata:
  name: ceph-client-conf
  namespace: openstack
stringData:
  ceph.client.openstack.keyring: |
    [client.openstack]
        key = <secret key>
        caps mgr = "allow *"
        caps mon = "profile rbd"
        caps osd = "profile rbd pool=images"
  ceph.conf: |
    [global]
    fsid = 7a1719e8-9c59-49e2-ae2b-d7eb08c695d4
    mon_host = 10.1.1.2,10.1.1.3,10.1.1.4
```

The following represents an example of Glance resource that can be used
to trigger the service deployment, and enable an RBD backend that
points to an external Ceph cluster.

```
apiVersion: glance.openstack.org/v1beta1
kind: Glance
metadata:
  name: glance
spec:
  serviceUser: glance
  containerImage: quay.io/tripleozedcentos9/openstack-glance-api:current-tripleo
  customServiceConfig: |
    [DEFAULT]
    enabled_backends = default_backend:rbd
    [glance_store]
    default_backend = default_backend
    [default_backend]
    rbd_store_ceph_conf = /etc/ceph/ceph.conf
    store_description = "RBD backend"
    rbd_store_pool = images
    rbd_store_user = openstack
  databaseInstance: openstack
  databaseUser: glance
  glanceAPIInternal:
    debug:
      service: false
    preserveJobs: false
    replicas: 1
  glanceAPIExternal:
    debug:
      service: false
    preserveJobs: false
    replicas: 1
  secret: osp-secret
  storageClass: ""
  storageRequest: 1G
  extraMounts:
    - name: v1
      region: r1
      extraVol:
        - propagation:
          - Glance
          extraVolType: Ceph
          volumes:
          - name: ceph
            projected:
              sources:
              - secret:
                  name: ceph-client-conf
          mounts:
          - name: ceph
            mountPath: "/etc/ceph"
            readOnly: true
```

When the service is up and running, it will be possible to interact
with the Glance API and upload an image using the Ceph backend.

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

