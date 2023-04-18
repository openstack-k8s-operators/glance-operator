# GLANCE-OPERATOR

The glance-operator is an OpenShift Operator built using the
[Operator Framework for Go](https://github.com/operator-framework). The
Operator provides a way to install and manage the OpenStack Glance
installation on OpenShift. This Operator is developed using RDO containers
for OpenStack.

## Getting started

**NOTES:**

- *The project is in a rapid development phase and not yet intended for
  production consumption, so instructions are meant for developers.*

- *If possible don't run things in your own machine to avoid the risk of
  affecting the development of your other projects.*

Here we'll explain how to get a functiona OpenShift deployment running inside a
VM that is running MariaDB, RabbitMQ, KeyStone and Glance services
against a Ceph backend.

There are 4 steps:

- [Install prerequisites](#prerequisites)
- [Deploy an OpenShift cluster](#openshift-cluster)
- [Prepare Storage](#storage)
- [Deploy OpenStack](#deploy)

### Prerequisites

There are some tools that will be required through this process, so the first
thing we do is install them:

```sh
sudo dnf install -y git wget make ansible-core python-pip podman gcc
```

We'll also need this repository as well as `install_yamls`:

```sh
cd ~
git clone https://github.com/openstack-k8s-operators/install_yamls.git
git clone https://github.com/openstack-k8s-operators/glance-operator.git
```

### OpenShift cluster

There are many ways get an OpenShift cluster, and our recommendation for the
time being is to use [OpenShift Local](https://access.redhat.com/documentation/en-us/red_hat_openshift_local/2.5/html/getting_started_guide/index)
(formerly known as CRC / Code Ready Containers).

To help with the deployment we have [companion development tools](https://github.com/openstack-k8s-operators/install_yamls/blob/master/devsetup)
available that will install OpenShift Local for you and will also help with
later steps.

Running OpenShift requires a considerable amount of resources, even more when
running all the operators and services required for an OpenStack deployment,
so make sure that you have enough resources in the machine to run everything.

You will need at least 5 CPUS and 16GB of RAM, preferably more, just for the
local OpenShift VM.

**You will also need to get your [pull-secrets from Red Hat](
https://cloud.redhat.com/openshift/create/local) and store it in the machine,
for example on your home directory as `pull-secret`.**

```sh
cd ~/install_yamls/devsetup
PULL_SECRET=~/pull-secret CPUS=6 MEMORY=20480 make download_tools crc
```

This will take a while, but once it has completed you'll have an OpenShift
cluster ready.

Now you need to set the right environmental variables for the OCP cluster, and
you may want to logging to the cluster manually (although the previous step
already logs in at the end):

```sh
eval $(crc oc-env)
```

**NOTE**: When CRC finishes the deployment the `oc` client is logged in, but
the token will eventually expire, in that case we can login again with
`oc login -u kubeadmin -p 12345678 https://api.crc.testing:6443`.

Let's now get the cluster version confirming we have access to it:

```sh
oc get clusterversion
```

If you are running OCP on a different machine you'll need additional steps to
[access its dashboard from an external system](https://github.com/openstack-k8s-operators/install_yamls/tree/master/devsetup#access-ocp-from-external-systems).

### Storage

There are 2 kinds of storage we'll need: One for the pods to run, for example
for the MariaDB database files, and another for the OpenStack services to use
for the VMs.

To create the pod storage we run:

```sh
cd ~/install_yamls
make crc_storage
```

As for the storage for the OpenStack services, at the time of this writing only
File and Ceph are supported. The [Glance Spec](#example-configure-glance-with-ceph-backend)
can be used to configure Glance to connect to a Ceph RBD server.

## Deploy openstack-k8s-operators

Deploying the podified OpenStack control plane is a 2 step process. First
deploying the operators, and then telling the openstack-operator how we want
our OpenStack deployment to look like.

Deploying the openstack operator:

```sh
cd ~/install_yamls
make openstack
```

Once all the operator ready we'll see the pod with:

```sh
oc get pod -l control-plane=controller-manager
```

And now we can tell this operator to deploy RabbitMQ, MariaDB, Keystone and Glance
with File as a backend:

```sh
cd ~/install_yamls
make openstack_deploy
```

After a bit we can see the 4 operators are running:

```sh
oc get pods -l control-plane=controller-manager
```

And a while later the services will also appear:

```sh
oc get pods -l app=mariadb
oc get pods -l app.kubernetes.io/component=rabbitmq
oc get pods -l service=keystone
oc get pods -l service=glance
```

### Configure Clients

We can now see available endpoints and services to confirm that the clients and
the Keystone service work as expected:

```sh
openstack service list
openstack endpoint list
```

Upload a glance image:

```sh
cd
wget http://download.cirros-cloud.net/0.5.2/cirros-0.5.2-x86_64-disk.img -O cirros.img
openstack image create cirros --container-format=bare --disk-format=qcow2 < cirros.img
openstack image list
```

## Cleanup

To delete the deployed OpenStack we can do:

```sh
cd ~/install_yamls
make openstack_deploy_cleanup
```

Once we've done this we need to recreate the PVs that we created at the start,
since some of them will be in failed state:

```sh
make crc_storage_cleanup crc_storage
```

We can now remove the openstack-operator as well:

```sh
make openstack_cleanup
```

# ADDITIONAL INFORMATION

**NOTE:** Run `make --help` for more information on all potential `make`
targets.

More information about the Makefile can be found via the [Kubebuilder
Documentation]( https://book.kubebuilder.io/introduction.html).

For developer specific documentation please refer to the [Contributing
Guide](CONTRIBUTING.md).

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

## Example: configure Glance with additional networks

The Glance spec can be used to configure Glance to have the pods
being attached to additional networks to e.g. connect to a Ceph
RBD server on a dedicated storage network.

Create a network-attachement-definition whic then can be referenced
from the Glance API CR.

```
---
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: storage
  namespace: openstack
spec:
  config: |
    {
      "cniVersion": "0.3.1",
      "name": "storage",
      "type": "macvlan",
      "master": "enp7s0.21",
      "ipam": {
        "type": "whereabouts",
        "range": "172.18.0.0/24",
        "range_start": "172.18.0.50",
        "range_end": "172.18.0.100"
      }
    }
```

The following represents an example of Glance resource that can be used
to trigger the service deployment, and have the service pods attached to
the storage network using the above NetworkAttachmentDefinition.

```
apiVersion: glance.openstack.org/v1beta1
kind: Glance
metadata:
  name: glance
spec:
  ...
  glanceAPIInternal:
    ...
    networkAttachents:
    - storage
  glanceAPIExternal:
    ...
    networkAttachents:
    - storage
...
```

When the service is up and running, it will now have an additional nic
configured for the storage network:

```
# oc rsh glance-external-api-dfb69b98d-mbw42
Defaulted container "glance-api" out of: glance-api, init (init)
sh-5.1# ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
3: eth0@if298: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1450 qdisc noqueue state UP group default
    link/ether 0a:58:0a:82:01:18 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.130.1.24/23 brd 10.130.1.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::4cf2:a3ff:feb0:932/64 scope link
       valid_lft forever preferred_lft forever
4: net1@if26: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether a2:f1:3b:12:fd:be brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 172.18.0.52/24 brd 172.18.0.255 scope global net1
       valid_lft forever preferred_lft forever
    inet6 fe80::a0f1:3bff:fe12:fdbe/64 scope link
       valid_lft forever preferred_lft forever
```

## Example: expose Glance to an isolated network

The Glance spec can be used to configure Glance to register e.g.
the internal endpoint to an isolated network. MetalLB is used for this
scenario.

As a pre requisite, MetalLB needs to be installed and worker nodes
prepared to work as MetalLB nodes to serve the LoadBalancer service.

In this example the following MetalLB IPAddressPool is used:

```
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: osp-internalapi
  namespace: metallb-system
spec:
  addresses:
  - 172.17.0.200-172.17.0.210
  autoAssign: false
```

The following represents an example of Glance resource that can be used
to trigger the service deployment, and have the internal glanceAPI endpoint
registerd as a MetalLB service using the IPAddressPool `osp-internal`,
request to use the IP `172.17.0.202` as the VIP and the IP is shared with
other services.

```
apiVersion: glance.openstack.org/v1beta1
kind: Glance
metadata:
  name: glance
spec:
  ...
  glanceAPIInternal:
    ...
    externalEndpoints:
      - endpoint: internal
        ipAddressPool: osp-internalapi
        loadBalancerIPs:
        - "172.17.0.202"
        sharedIP: true
        sharedIPKey: ""
    ...
...
```

The internal glance endpoint gets registered with its service name. This
service name needs to resolve to the `LoadBalancerIP` on the isolated network
either by DNS or via /etc/hosts:

```
# openstack endpoint list -c 'Service Name' -c Interface -c URL --service glance
+--------------+-----------+---------------------------------------------------------------+
| Service Name | Interface | URL                                                           |
+--------------+-----------+---------------------------------------------------------------+
| glance       | internal  | http://glance-internal.openstack.svc:9292                     |
| glance       | public    | http://glance-public-openstack.apps.ostest.test.metalkube.org |
+--------------+-----------+---------------------------------------------------------------+
```

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
