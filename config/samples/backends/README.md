# Glance Backend/Store Samples

This directory includes a set of Glance Store/Backend configuration samples
that use the `kustomize` configuration management tool available through the `oc
kustomize` command.

These samples are not meant to serve as deployment recommendations, just as
working examples to serve as reference.

For each backend/store there will be a `backend.yaml` file containing an
overlay for the `OpenStackControlPlane` with just the storage related
information.

Backend pre-requirements will be listed in that same `backend.yaml` file.
These can range from having to replace the storage system's address and
credentials in a different yaml file, to having to create secrets.

Currently available samples are:

- Ceph
- NFS
- CEPH + NFS
- CEPH + Sparse Image Upload
- Cinder backends
- Swift

The following Cinder backend examples are available:

- Cinder using LVM iSCSI
- Cinder backend using LVM NVMe-TCP

For these the file structure is different, as the glance configuration is the
same for them all and only the Cinder configuration changes.

The base Glance configuration to use Cinder is stored in
`./cinder/glance-common` and the different Cinder configurations will be in
the other directories under `./cinder`.

## Ceph example

Assuming you are using `install_yamls` and you already have `crc` running you
can use the Ceph example with:

```
$ cd install_yamls
$ make ceph TIMEOUT=90
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/backends/ceph > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

If we already have a deployment working we can always use
`oc kustomize ceph | oc apply -f -`. from this directory to make the changes.

**Note:**

When Ceph is adopted as a backend, Glance `image-conversion` is enabled by default.
It's realized through a dedicated `PVC` (built by the `StatefulSet` via templates)
that is mounted to the `/var/lib/glance/os_glance_staging_store` path.
A `glance-conversion` PVC can be found inspecting the Glance **external** Pod
via the `oc describe pod .. ` command:


```bash
...
    Mounts:
      /etc/ceph from ceph (ro)
      /etc/my.cnf from config-data (ro,path="my.cnf")
      /usr/local/bin/container-scripts from scripts (ro)
      /var/lib/config-data/default from config-data (ro)
      /var/lib/glance from glance (rw)
      /var/lib/glance/os_glance_staging_store from glance-conversion (rw)
      /var/lib/kolla/config_files/config.json from config-data (ro,path="glance-api-config.json")
      /var/log/glance from logs (rw)
...
```
The PVC is only created for the external instance: this space is only used to
store staging data of the image that is going to be uploaded, and an internal
`glanceAPI` will never use it.

## Ceph with Sparse Image Upload example

Assuming you are using `install_yamls` and you already have `crc` running you
can use the Ceph example with:

```
$ cd install_yamls
$ make ceph TIMEOUT=90
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/backends/ceph_thin_provisioning > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

If we already have a deployment working we can always use
`oc kustomize ceph | oc apply -f -`. from this directory to make the changes.

## Cinder examples

Once we have `crc` running making a deployment with Cinder as a backend is
trivial:

```
$ cd install_yamls
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/backends/cinder/lvm-iscsi > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

Be aware that the Cinder examples will reboot your OpenShift cluster because
they use `MachineConfig` manifests that require a reboot to be applied.  This
means that the deployment takes longer and the cluster will stop responding for
a bit.

## Swift example

Once `crc` is up and running you can build an OpenStack control plane with
Swift as a backend:

```
$ cd install_yamls
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/backends/swift > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```
In case RGW is used in place of swift, it's possible to reuse the same `Glance`
configuration to interact with an `object-store` endpoint that points to an RGW
instance.
A variation of the procedure described above allows to deploy `Glance` with a
`Swift` backend where behind the scenes `RGW` acts as `object-store` backend:

```
$ cd install_yamls
$ make ceph TIMEOUT=90
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/backends/swift > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

Before start using `Glance` with `RGW` in place of `Swift`, a few additional
resources should be created in the deployed control plane. Run the following
commands on an already deployed OpenStack control plane to create users and
roles as they will be used by the RGW instances to interact with keystone.

```
openstack service create --name swift --description "OpenStack Object Storage" object-store
openstack user create --project service --password $SWIFT_PASSWORD swift
openstack role create swiftoperator
openstack role create ResellerAdmin
openstack role add --user swift --project service member
openstack role add --user swift --project service admin

export RGW_ENDPOINT=192.168.122.3
for i in public internal; do
    openstack endpoint create --region regionOne object-store $i http://$RGW_ENDPOINT:8080/swift/v1/AUTH_%\(tenant_id\)s;
done

openstack role add --project admin --user admin swiftoperator
```

- Replace `$SWIFT_PASSWORD` with the password that should be assigned to the swift user.
- Replace 192.168.122.3 with the IP address reserved as `$RGW_ENDPOINT`. If
  network isolation is used make sure the reserved address can be reached by the
  swift client that starts the connection.

Additional details on the `Ceph RGW` configuration are described in the
[openstack-k8s-operators/docs repo](https://github.com/openstack-k8s-operators/docs/blob/main/ceph.md#configure-swift-with-a-rgw-backend).


## NFS Example

It is possible to configure Glance with a NFS backend.
However, unlike Cinder, Glance has no notion of a NFS backend, and it relies on
the File driver when this strategy is chosen.
Glance does not recommend that you use NFS storage because its capabilities are
limited compared to the other backends like Ceph, Cinder, Swift.
To configure Glance with the NFS backend, the top-level CR exposes the required
k8s parameters via the [ExtraMounts](https://github.com/openstack-k8s-operators/docs/blob/main/extra_mounts.md)
feature, and it assumes a NFS export already exists and is reachable by the
OpenStack control plane.


### Configure the NFS backend

Create the GlanceCR, and use extraMounts to add both the IP address and the path
of the NFS share: it's mapped to `/var/lib/glance/images`, path used by the
GlanceAPI service to store and retrieve the images:

```
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack
spec:
...
...
  extraMounts:
  - extraVol:
    - extraVolType: Nfs
    mounts:
    - mountPath: /var/lib/glance/images
      name: nfs
    propagation:
    - Glance
    volumes:
    - name: nfs
      nfs:
        path: {{ NFS_EXPORT_PATH }}
        server: {{ NFS_IP_ADDRESS }}
```

#### Note

The `NFS_IP_ADDRESS` must be part of the overlay network reachable by Glance.
In general, when the OpenStack control plane is created, a
`NodeNetworkConfigurationPolicy (nncp)` CR file is generated and applied, as
well as the corresponding `NetworkAttachmentDefinition (net-att-def)` CR that
defines the resources for the isolated network where the NFS share is exported.
In a deployed OpenStack control plane, you can check both the interfaces
(`nncp`) and the `NetworkAttachmentDefinition(s)` with the following commands:

```
$ oc get nncp
NAME                        STATUS      REASON
enp6s0-crc-8cf2w-master-0   Available   SuccessfullyConfigured

$ oc get net-attach-def

NAME
ctlplane
internalapi
storage
tenant

$ oc get ipaddresspool -n metallb-system

NAME          AUTO ASSIGN   AVOID BUGGY IPS   ADDRESSES
ctlplane      true          false             ["192.168.122.80-192.168.122.90"]
internalapi   true          false             ["172.17.0.80-172.17.0.90"]
storage       true          false             ["172.18.0.80-172.18.0.90"]
tenant        true          false             ["172.19.0.80-172.19.0.90"]
```


### Deploy Glance with a NFS backend

Assuming you are using `install_yamls` and you already have a `crc` environment
running you can deploy `Glance` with the NFS backend with the following commands:

```
$ cd install_yamls
$ pushd devsetup; make crc_attach_default_interface; popd
$ make crc_storage openstack
$ ssh -i ~/.crc/machines/crc/id_ecdsa -o StrictHostKeyChecking=no core@`crc ip`
$ curl -O https://raw.githubusercontent.com/openstack-k8s-operators/glance-operator/main/config/samples/backends/nfs/create_export.sh
$ NFS_NET_INTERFACE=enp6s0.21 ./create_export.sh
$ oc kustomize ../glance-operator/config/samples/backends/nfs > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

When `GlanceAPI` is active, you can see a single API instance:

```
$ oc get pods -l service=glance

NAME                      READY   STATUS    RESTARTS
glance-default-single-0   3/3     Running   0
```

and the description of the pod reports:

```
Mounts:
...
  nfs:
    Type:      NFS (an NFS mount that lasts the lifetime of a pod)
    Server:    172.18.0.5
    Path:      /var/nfs
    ReadOnly:  false
...
```

It is also possible to double check the mount point by running the following:

```
oc rsh -c glance-api glance-default-single-0
sh-5.1# mount
...
...
172.18.0.5:/var/nfs on /var/lib/glance/images type nfs4 (rw,relatime,vers=4.2,rsize=1048576,wsize=1048576,namlen=255,hard,proto=tcp,timeo=600,retrans=2,sec=sys,clientaddr=172.18.0.5,local_lock=none,addr=172.18.0.5)
...
...
```

At this point, you can run an `openstack image create` command and double check,
on the NFS node, the `uuid` has been created in the exported directory:


```
$ oc rsh openstackclient
$ openstack image list

sh-5.1$  curl -L -o /tmp/cirros-0.5.2-x86_64-disk.img http://download.cirros-cloud.net/0.5.2/cirros-0.5.2-x86_64-disk.img
...
...

sh-5.1$ openstack image create --container-format bare --disk-format raw --file /tmp/cirros-0.5.2-x86_64-disk.img cirros
...
...

sh-5.1$ openstack image list
+--------------------------------------+--------+--------+
| ID                                   | Name   | Status |
+--------------------------------------+--------+--------+
| 634482ca-4002-4a6d-b1d5-64502ad02630 | cirros | active |
+--------------------------------------+--------+--------+
```

and, on the crc node, we can see the same `uuid` in the exported `/var/nfs`:

```
$ ssh -i ~/.crc/machines/crc/id_ecdsa -o StrictHostKeyChecking=no core@`crc ip`
$ ls /var/nfs/
634482ca-4002-4a6d-b1d5-64502ad02630
```


## Multistore

It is possible to configure multiple backends (known as `stores`) for a single
GlanceAPI instance.
This is the case of `multistore`: `enabled_backends` must be set as a `key:value`
pair, where:
- key: represents the identifier for the store
- value: represents the type of the store (valid values are `file`, `http`, `rbd`,
  `swift`, `cinder`).

More information around multistore configuration can be found in the [upstream](https://docs.openstack.org/glance/latest/admin/multistores.html)
documentation.
The `glance-operator` provides an example of `OpenStackControlPlane` CR that
provides three stores:

```bash
...
customServiceConfig: |
  [DEFAULT]
  debug=True
  enabled_backends = ceph-0:rbd,ceph-1:rbd,swift-0:swift
```

To deploy the multistore sample file, run the following commands:

```bash
$ cd install_yamls
$ CEPH_CLUSTERS=2 TIMEOUT=120 make ceph
```

The commands above will generate two Ceph clusters and exports the associated
secrets.

```bash
$ oc get pods -l app=ceph
NAME     READY   STATUS    RESTARTS
ceph-0   1/1     Running   0
ceph-1   1/1     Running   0

$ oc get secret -l app=ceph
ceph-conf-files-0
ceph-conf-files-1
```

At this point, deploy the `OpenStackControlPlane` using the [multistore samples](https://github.com/openstack-k8s-operators/glance-operator/tree/main/config/samples/backends/multistore).

```bash
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/backends/multistore > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

Once the `OpenStackControlPlane` is ready, it is possible to upload an image on
a particular store, or just upload an image to the three of them (useful for testing
purposes).
To list the available stores, run the following command:

```bash
$ glance --os-auth-url $keystone --os-project-name admin --os-username admin \
         --os-password $password --os-user-domain-name default \
         --os-project-domain-name default stores-info

+----------+----------------------------------------------------------------------------------+
| Property | Value                                                                            |
+----------+----------------------------------------------------------------------------------+
| stores   | [{"id": "ceph-0", "description": "RBD backend"}, {"id": "ceph-1", "description": |
|          | "RBD backend 1", "default": "true"}, {"id": "swift-0"}]                          |
+----------+----------------------------------------------------------------------------------+
```

To upload an image (e.g., cirros) on a particular store, for instance `swift-0`,
run the following command:

```bash
$ glance --os-auth-url $keystone --os-project-name admin --os-username admin \
         --os-password $password --os-user-domain-name default \
         image-create-via-import --store swift-0 --container-format bare \
         --disk-format raw \
         --uri http://download.cirros-cloud.net/0.5.2/cirros-0.5.2-x86_64-disk.img \
         --import-method web-download --name cirros
```

To upload an image (e.g., cirros) on all the stores, run the following command:

```bash
$ glance --os-auth-url $keystone --os-project-name admin --os-username admin \
         --os-password $password --os-user-domain-name default \
         image-create-via-import --all-stores true --container-format bare \
         --disk-format raw --uri http://download.cirros-cloud.net/0.5.2/cirros-0.5.2-x86_64-disk.img \
         --import-method web-download --name cirros
```


## Adding new samples

We are open to PRs adding new samples for other backends.

Most backends will require credentials to access the storage, usually there are
2 types of credentials:

- Configuration options in `glance-api.conf`
- External files

You can find the right approach to each of them in the `nfs` sample (for
configuration parameters) and the `ceph` sample (for providing files).
