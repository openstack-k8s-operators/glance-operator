# Running the operator locally

**NOTE**: This article makes some assumptions about your environment setup,
make sure they are correct or adapt the steps accordingly.

This development model is useful for quick iterations of the operator code
where one can easily use debugging tools and change template and asset files on
the fly without needed for deployments.

We will build and run the operator on the host machine that is running the
OpenShift VM and it will connect to the OpenShift cluster from the outside
using our credentials.

The downside of this approach is that we are not running things in a container
inside OpenShift, so there could be differences between what is accessible in
one case and the other.  For example, we'll have admin credentials running
things on the host, whereas the operator deployed inside OpenShift will have
more restrictive ACLs.

Another downside is that we'll have to manually login into the cluster every
time our login credentials expire.

### Preparation

This article assumes we have followed the [Getting
Started](https://github.com/openstack-k8s-operators/glance-operator/blob/main/README.md#getting-started)
section successfully so we'll not only have a glance-operator pod running,
but also other required services running.

Since we have everything running we need to uninstalling both the
glance-operator and Glance services.

To uninstall the Glance services we will edit the `OpenStackControlPlane` CR
that we used to deploy OpenStack and is present in the OpenShift cluster.

```sh
oc edit OpenStackControlPlane openstack
```

Now we search for the `glance` section and in its `template` section we change
the `replicas` value to `0` then save and exit the editor.

This will make the openstack-operator notice the change and modify the `Glance`
CR, which in turn will be detected by the glance-operator triggering the
termination of the glance services in order during the reconciliation.

**NOTE**: The Glance DB is not deleted when uninstalling Glance services, so
Glance DB migrations will run faster on the next deploy (they won't do
anything) and images or any other records will not be lost.

Once we no longer have any of the glance service pods (`oc get pod -l
service=glance` returns no pods) we can proceed to remove the glance-operator
pod that is currently running on OpenShift so it doesn't conflict with the one
we'll be running locally.

We search for the name of the `ClusterServiceVersion` of the OpenStack operator
and edit its current CR:

```sh
CSV=`oc get -l operators.coreos.com/openstack-operator.openstack= -o custom-columns=CSV:.metadata.name --no-headers csv`

oc edit csv $CSV
```

This will drop us in our editor with the contents of CSV YAML manifest where
we'll search for the first instance of `name:
glance-operator-controller-manager`, and we should see something like:

```
      - label:
          control-plane: controller-manager
        name: glance-operator-controller-manager
        spec:
          replicas: 1
          selector:
            matchLabels:
              control-plane: controller-manager
```

Where we see `replicas: 1` change it to `replicas: 0`, save and exit. This
triggers the termination of the glance-operator pod.

### Build and Run

Before continuing make sure you are in the `glance-operator` directory where
the changes to test are.

If our local code is changing the glance-operator CRDs or adding new ones we
need to regenerate the manifests and change them in OpenShift.  This can be
easily done by running `make install`, which first builds the CRDs (using the
`manifests` target) and then installs them in the OpenShift cluster.

Now it's time to build the glance operator (we'll need go version 1.18) and run
it (remember you need to be logged in with `oc login` or `crc_login` if you are
using the helper functions):

```sh
make build
make run
```

Any changes in the `templates` directory will be automatically available to the
operator and there will be no need to recompile, rebuild, or restart the
glance-operator.

Now that the glance operator is running locally we can go back and set the
`replicas` back to `1` in the `glance` section of the `OpenStackControlPlane`
CR to trigger the deployment of the Glance services.

We should now see the local glance-operator detecting the change and we'll be
able to see validate our code changes.

### Final notes

If there's something wrong we can stop the operator with Ctrl+C and repeat the
process: Run `make install` if there are changes to the CRDs and rebuild and
rerun the glance-operator.
