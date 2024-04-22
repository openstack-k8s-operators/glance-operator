# Distributed image import support

## Description

When images are uploaded via the import mechanism, they are stored in a special area called `staging`.
When Glance is deployed using multiple `API` worker nodes (which means `replicas > 1` in a kubernetes
related environment), the staging directories of all worker nodes are not shared (i.e. they’re not
mounted on a common `NFS server` or `RWX` pvc).
Each Pod, and in particular each replica has its own `RWO` `PVC`, which marks them as isolated from
each other. As `glance-api` service is fully independent and stateless,  in the sense that there’s no
clustering knowledge at `Glance` level  (each API/replica is able to act as a standalone entity), the
different nodes do not know nor care if they are operating alone or in a cluster.
This poses challenges to address some of the distributed models, and the `distributed-image-import`
feature builds that missing understanding between nodes, and adds that knowledge so that there are other
nodes in the same environments that might have different needs or resources available.


## distributed image import workflow

In order to get an image from zero to usable, the distributed image import uses the `glance-direct`
[import method]().

Two main `API` requests are required in this scenario:
- **staging** of the image data: the image is imported into a staging directory
  configured on the current worker that receives the request
- **Import** operation:  it moves the data from the staging area to its final
  destination(s).

In a multi-node load-balanced scenario, there are high chances that _stage_ and
_import_ operations  hit different workers: in a normal context this would
result in the latter not having access to the staged image data in its staging
store, generating a failure.
However, a critical aspect of this flow, that actually solves the problem
described above, is the ability to **proxy** the import request to the node
where the image has been staged already, and it allows the API workers to keep
their staging store directories local and unshared (using a local `RWO` Pvc for
each replica).
To be able to _proxy_ the import request to the right node, when the image is
created and staged, `Glance` records the `URL` (in the database) by which the
staging worker can be reached from the other workers: with this information all
the proxy requests (both import and stage-delete) can be forwarded to the right
worker that has the data and that is able to perform the associated action
successfully. With the above flow, `Glance` eliminates the need for shared
storage between the `API` worker nodes, allowing them to be isolated from an
High Availability point of view, as well as distributed geographically (a `DCN`
environment can take advantage of this feature).


```
+---------+                                                        +-------------------+         +-------------------+                  +-------------------+
| Client  |                                                        | GlanceAPIWorkerEP |         | GlanceAPIWorker0  |                  | GlanceAPIWorker1  |
+---------+                                                        +-------------------+         +-------------------+                  +-------------------+
     |                                                                       |                             |                                      |
     | image-create(img_name)                                                |                             |                                      |
     |---------------------------------------------------------------------->|                             |                                      |
     |                                                                       |                             |                                      |
     |                                                        ack/nack(uuid) |                             |                                      |
     |<----------------------------------------------------------------------|                             |                                      |
     |                                                                       |                             |                                      |
     | glance image-stage --progress --file $IMG_FILE $IMG_UUID              |                             |                                      |
     |---------------------------------------------------------------------->|                             |                                      |
     |                                                                       |                             |                                      |
     |                                                                       | image_stage(uuid)           |                                      |
     |                                                                       |---------------------------->|                                      |
     |                                                                       |                             |                                      |
     |                                                                       |                    ack/nack |                                      |
     |<----------------------------------------------------------------------------------------------------|                                      |
     |                                                                       |                             |                                      |
     | glance image-import --import-method glance-direct $IMG_UUID           |                             |                                      |
     |---------------------------------------------------------------------->|                             |                                      |
     |                                                                       |                             |                                      |
     |                                                                       | image_import(uuid)          |                                      |
     |                                                                       |------------------------------------------------------------------->|
     |                                                                       |                             |                                      |
     |                                                                       |                             |                                      | worker_node = inspect_worker_self_reference_url_db(uuid)
     |                                                                       |                             |                                      |---------------------------------------------------------
     |                                                                       |                             |                                      |                                                        |
     |                                                                       |                             |                                      |<--------------------------------------------------------
     |                                                                       |                             |                                      |
     |                                                                       |                             |           proxy_import_request(uuid) |
     |                                                                       |                             |<-------------------------------------|
     |                                                                       |                             |                                      |
     |                                                                       |                             | process_import_task(uuid)            |
     |                                                                       |                             |--------------------------            |
     |                                                                       |                             |                         |            |
     |                                                                       |                             |<-------------------------            |
     |                                                                       |                             |                                      |
     |                                                                       |                    ack/nack |                                      |
     |<----------------------------------------------------------------------------------------------------|                                      |
     |                                                                       |                             |                                      |




```


The picture above shows how, in a scenario where we scaled up a `GlanceAPI` to
`replicas: 2`, how a potential image import flow would work, and how `GlanceAPI`
worker 1, which represents the additional replica, would resolve the url of the
worker where the image data are staged, and properly proxy the import request.


## Glance operator support

With the theory described in the previous sections, the goal is to enable the
`glance-operator` to properly support this feature, and this means trying to
find the proper solution that solves many challenges that didn't represent an
issue in the baremetal world.
In the deployer based on `TripleO`, when a cloud administrators enables the
`glance-direct` image import method, users can upload local images to `Glance`
without any requirement for a shared staging area, and the staging is
distributed because individual `GlanceAPI` workers keep their staging store
directories _local_ and _unshared_ but still perform image import operations.
In `TripleO` enabling this feature usually requires two actions:
- Add GlanceEnabledImportMethods: glance-direct,web-download among the
  environment parameters
- Configure each API worker with a URL by which other API workers can reach it
  directly.

This configuration allows one worker behind a load balancer to stage an image
when the first request occurs, and a different worker to handle the import
request and properly proxy the request. `TripleO` enables it via
[openstack/tripleo-heat-templates/+/882391](https://review.opendev.org/c/openstack/tripleo-heat-templates/+/882391)

In `glance-operator`, enabling this feature means solving, in the first place,
the following challenges:
- Properly route the requests from the external endpoint, used by a Client that
  initiates the connection, to the Pod that actually owns the staged data
- Provide a unique network Identifier that is always valid at replica level,
  even though a given Pod is deleted and rescheduled on a different worker
  node, resulting in a change in its cluster IP address
- Set the right `worker_self_reference_url` (at runtime) to make sure it always
  identify the right replica which owns the staged image data

**Note**:

Cluster IP address are defined using openshift-sdn (as known as cluster
network), and such network is used for pod to pod communication even in a
network isolation scenario

### StatefulSet

Unlike a `Deployment`, a `StatefulSet` maintains a `sticky` identity for each of
its Pods. These `Pods` are created from the same spec (which means same config
across Pods and replicas), but are not interchangeable: each has a persistent
identifier that it maintains across any rescheduling.
The `StatefulSet` represents the first critical component, because it provides:

- Stable, unique **network identifiers**.
- Stable, **persistent storage**.
- Ordered, graceful deployment and scaling.
- Ordered, automated rolling updates.

However, `StatefulSets` currently require a `Headless Service` to be responsible
for the network identity of the Pods.

### Kubernetes Headless Service

For `Headless Services`, a cluster IP is not allocated, `kube-proxy` does not
handle these `Services`, and there is no load balancing or proxying done by the
platform for them. How `DNS` is automatically configured depends on whether the
Service has selectors defined.
`Kubernetes` allows clients to discover `Pod IPs` through `DNS` lookups.
When a `ClusterIP` is set, if a `DNS` lookup for a service is performed, the
DNS server returns a single IP Address, which represents the service’s cluster
IP.

```bash
$ oc get svc | grep -i keystone-internal
keystone-internal         ClusterIP   10.217.5.14    <none>        5000/TCP                       10d

/ # nslookup keystone-internal
Server:    10.217.4.10
Address 1: 10.217.4.10 dns-default.openshift-dns.svc.cluster.local

Name:      keystone-internal
Address 1: 10.217.5.14 keystone-internal.openstack.svc.cluster.local
```

However, it is possible to not reserve a `ClusterIP` for a given service: in
this case the `DNS` server will return all the `Pod IPs` instead of the single
service IP.
Instead of returning a single `DNS A` record, the DNS server will return multiple
`A` records for the service, each pointing to the IP of an individual pod backing
the service.

```bash
$ oc get svc
NAME                      TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                        AGE
glance-default-external-api   ClusterIP   None           <none>        9292/TCP
glance-default-internal-api   ClusterIP   None           <none>        9292/TCP

/ # nslookup glance-default-external-api
Server:    10.217.4.10
Address 1: 10.217.4.10 dns-default.openshift-dns.svc.cluster.local

Name:      glance-default-external-api
Address 1: 10.217.1.252 glance-default-external-api-0.glance-default-external-api.openstack.svc.cluster.local
Address 2: 10.217.0.30 glance-default-external-api-1.glance-default-external-api.openstack.svc.cluster.local
Address 3: 10.217.0.39 glance-default-external-api-2.glance-default-external-api.openstack.svc.cluster.local
```

Clients can therefore do a simple `DNS A` record lookup and get the IPs of all
the Pods that are part of the service. The client can then use that information
to connect to one, many, or all of them.

```bash
sh-5.1(glance-default-external-0) >
# curl glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292
{"versions": [{"id": "v2.15", "status": "CURRENT", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.13", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.12", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.11", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.10", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.9", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.8", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.7", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.6", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.5", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.4", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]}, {"id": "v2.3", "status": "SUPPORTED", "links": [{"rel": "self", "href": "https://glance-default-external-api-2.glance-default-external.openstack.svc.cluster.local:9292/v2/"}]},
...
...
```


Setting the `clusterIP` field in a service `spec` to `None` makes the service
headless, and Kubernetes won’t assign a cluster IP through which clients could
connect to the pods backing it. This results in bypassing `kube-proxy`, a
component that translates `Services` into some networking rules, used to
redistribute the traffic to the Pods behind the service accordingly. In the
openstack control plane, overrides are usually passed for the internal endpoints
exposed for a particular service (and that are typically registered in keystone):
the strategy to get a working headless service without affecting the whole
deployment is **to create, for each statefulset, an associated headless service
that can only be used for pod2pod communication**. A generic service layout (for
a `Ceph` based configuration or any other relevant backend) looks like the
following:

```bash
## SVCs

NAME                          TYPE      CLUSTER-IP       EXTERNAL-IP   PORT(S)
glance-default-external-api   ClusterIP   None           <none>        9292/TCP
glance-default-internal       ClusterIP   10.217.5.11    <none>        9292/TCP
glance-default-internal-api   ClusterIP   None           <none>        9292/TCP
glance-default-public         ClusterIP   10.217.4.191   <none>        9292/TCP

## Pods

NAME                            READY   STATUS    RESTARTS
glance-default-external-api-0   3/3     Running   0
glance-default-internal-api-0   3/3     Running   0

## GlanceAPIs

NAME                          NETWORKATTACHMENTS   STATUS   MESSAGE
glance-default-external       True                 Setup    complete
glance-default-internal       True                 Setup    complete

## StatefulSets

NAME                          READY
glance-default-external-api   1/1
glance-default-internal-api   1/1
```

### kube-proxy
Usually `kube-proxy` realizes this mapping through `NAT` rules inside the node.
These `NAT` rules are simply mappings from `Service IP` to `Pod IP`.
The NAT rules pick one of the Pods randomly. However, this behavior might
change depending on the Kube-Proxy “mode”.
Three modes are available:
- **Iptables mode**: the default and most used approach, with the drawback of
  relying on a sequential approach going through its tables (_O(n)_ as long as
  the rules increase)
- **IPVS mode**:  this is more efficient and also supports different load balancing
  algorithms like round robin, least connections, and other hashing approaches
  (_O(1)_, which means stable performances as long as the rules increase)
- **KernelSpace mode (VFP)**


### Final Glance Configuration


As per the other regular configuration options, It is possible to use
`customServiceConfig` to inject any additional configuration, but in general,
`glance-direct` import method is **enabled by default** and available for all
the `GlanceAPI` instances (and any instance that might be added later in time).


```bash
  customServiceConfig: |
    [DEFAULT]
    debug=True
    enabled_backends = default_backend:rbd
    ...
  ...
```

Assuming the `glance-operator` is able to deploy an `headless service`, when the
`StatefulSet` starts, `kolla_start` is executed to perform both the scripts and
config copy to the `glance.conf.d` target directory, and it executes any
additional step to properly run the main `glance-api` process.
The `glance-operator` is also able to provide the `Service` domain (defined in
the `CR status` field), and it's used to resolve the Image Service component
Pod IPs associated to a Service.
the kolla bootstrap process is responsible to execute the `kolla_extend_start`
script and defer the `worker_self_reference_url` setting to this stage.


```bash
-> "/usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start"
  -> /usr/local/bin/kolla_extend_start

GLANCE_PORT=${GLANCE_PORT:-9292}
GLOBAL_CONF=${GLOBAL_CONF:-/etc/glance/glance.conf.d/01-config.conf}

function set_worker_self_url {
cat <<EOF > "${GLOBAL_CONF}"
[DEFAULT]
worker_self_reference_url=$(hostname).${GLANCE_DOMAIN}:${GLANCE_PORT}
EOF
```

Where:

- 01-config.conf is not currently set by the operator (only 00-config.conf, 02-config.conf, 03-config.conf are provided), and as per the service config bootstrap guidelines 01 fits well the purpose of setting default global service config options
- GLANCE_DOMAIN is provided by the Stateful and it corresponds to the the Service hostname associated with a particular GlanceAPI
- GLANCE_PORT is usually 9292 but can be passed as parameter
- hostname: the hostname of the Glance Pod / Replica


## How to test

Assuming a given `GlanceAPI` instance has been scaled up, it is possible to
perform a `per-staged` test and verify that `Pods` are able to see each other
(or resolve them by `hostname`) and the request is properly `proxied` to the
replica that owns the data in its staging directory.

### Step 1: Create an empty box

The first step is to create an empty box that will contain the image once is
uploaded through the import command.

```bash
  glance --verbose image-create \
   --disk-format qcow2 \
   --container-format bare \
   --name $IMAGE_NAME
```

- **IMAGE_NAME**: the name of the image that is going to be imported through
  the glance-direct method

### Step 2: Stage the image

```bash
UUID=$(openstack image show $IMAGE_NAME -c id -f value)
glance image-stage --progress --file $IMAGE_FILE $UUID
```

- **IMAGE_FILE**: the path of the image file that should be staged on a
  glanceAPI replica


## Step 3: Import the image through the glance-direct method

Assuming the staging operation happened on `replica-0`, which owns the data in
its staging directory (`/var/lib/glance/os_staging_store`), the idea is to
perform the import operation using a `$replica` that has no data in its
`os_staging_store` directory: we can verify how the request is properly proxied
to the node/pod/replica that owns the data.

```bash
UUID=$(openstack image show $IMAGE_NAME -c id -f value)
glance --os-image-url $replica image-import --import-method glance-direct $ID
```
- **IMAGE_NAME**: the name of the image box that will be backed by the imported
  data

- **REPLICA**: a glanceAPI replicas that doesn’t own the data

## EXAMPLE

The example assumes a glanceAPI is deployed using [single layout](https://github.com/openstack-k8s-operators/glance-operator/tree/main/config/samples/layout/single).

Patch the resulting glanceAPI and scale it to `replicas:2`:

```
oc patch Glance glance --type=json -p="[{'op': 'replace', 'path': '/spec/glanceAPIs/default/replicas', value: 2}]"
```

Copy the [`image-import.sh`](import-image.sh) script to the target container
where the `glance` cli is available, for example one of the existing replicas:

```
oc cp import-image.sh -c glance-api glance-default-single-0:/
```

Run the script from the client Pod:

```bash
sh-5.1# export PASSWORD=12345678
sh-5.1# ./import-image.sh
+------------------+--------------------------------------+
| Property         | Value                                |
+------------------+--------------------------------------+
| checksum         | None                                 |
| container_format | bare                                 |
| created_at       | 2024-02-16T22:03:06Z                 |
| disk_format      | qcow2                                |
| id               | 6cc0be2c-82f9-423c-aae6-9baad4110c72 |
| locations        | []                                   |
| min_disk         | 0                                    |
| min_ram          | 0                                    |
| name             | myimage                              |
| os_hash_algo     | None                                 |
| os_hash_value    | None                                 |
| os_hidden        | False                                |
| owner            | 0179678e7fa04297afff4b41ad0b777d     |
| protected        | False                                |
| size             | None                                 |
| status           | queued                               |
| tags             | []                                   |
| updated_at       | 2024-02-16T22:03:06Z                 |
| virtual_size     | Not available                        |
| visibility       | shared                               |
+------------------+--------------------------------------+
ID: 6cc0be2c-82f9-423c-aae6-9baad4110c72

glance --os-auth-url http://keystone-public.openstack.svc:5000/v3 \
    --os-project-name admin
    --os-username admin
    --os-password 12345678
    --os-user-domain-name default
    --os-project-domain-name default
    image-stage --progress --file myimage
    6cc0be2c-82f9-423c-aae6-9baad4110c72

[=============================>] 100%

+-----------------------+--------------------------------------+
| Property              | Value                                |
+-----------------------+--------------------------------------+
| checksum              | None                                 |
| container_format      | bare                                 |
| created_at            | 2024-02-16T22:03:06Z                 |
| disk_format           | qcow2                                |
| id                    | 6cc0be2c-82f9-423c-aae6-9baad4110c72 |
| locations             | []                                   |
| min_disk              | 0                                    |
| min_ram               | 0                                    |
| name                  | myimage                              |
| os_glance_import_task | 4703634c-45d4-41b2-8d47-dd73b40a8689 |
| os_hash_algo          | None                                 |
| os_hash_value         | None                                 |
| os_hidden             | False                                |
| owner                 | 0179678e7fa04297afff4b41ad0b777d     |
| protected             | False                                |
| size                  | 22                                   |
| status                | uploading                            |
| tags                  | []                                   |
| updated_at            | 2024-02-16T22:03:11Z                 |
| virtual_size          | Not available                        |
| visibility            | shared                               |
+-----------------------+--------------------------------------+

+--------------------------------------+---------+
| ID                                   | Name    |
+--------------------------------------+---------+
| 6cc0be2c-82f9-423c-aae6-9baad4110c72 | myimage |
+--------------------------------------+---------+

STATUS: active
```
