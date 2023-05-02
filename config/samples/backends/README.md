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

## Ceph example

Once the OpenStack operators are running in your OpenShift cluster and
the secret `osp-secret` is present, one can deploy OpenStack with a
specific storage backend with single command.  For example for Ceph we can do:
`oc kustomize ceph | oc apply -f -`.

The result of the `oc kustomize ceph` command is a complete
`OpenStackControlPlane` manifest, and we can see its contents by redirecting it
to a file or just piping it to `less`: `oc kustomize ceph | less`.

Creating the basic secret that our samples require can be done using the
`install_yamls` target called `input`.

A complete example when we already have CRC running would be:

```
$ cd install_yamls
$ make ceph TIMEOUT=90
$ make crc_storage openstack input
$ cd ../glance-operator
$ oc kustomize config/samples/backends/ceph | oc apply -f -
```

## Adding new samples

We are open to PRs adding new samples for other backends.

Most backends will require credentials to access the storage, usually there are
2 types of credentials:

- Configuration options in `glance-api.conf`
- External files

You can find the right approach to each of them in the `nfs` sample (for
configuration parameters) and the `ceph` sample (for providing files).
