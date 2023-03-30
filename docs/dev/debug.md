# Debugging

When we deploy OpenStack using operators there are many moving pieces that must
work to get a running OpenStack deployment: OLM, OpenStack Operator, MariaDB
operator, RabbbitMQ operator, Keystone Operator, Glance Operator, etc. For that
reason it's good to know a bit about the different pieces and how they connect
to each other.

Besides reading this guide it is recommended to read the [Debug Running Pods
documentation](
https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod).

Usually the first step to resolve issues is to figure out **where** the issue
is happening and we can do it starting from the OLM and go forward through the
steps, do it in reverse starting from the cinder-operator and move backwards,
or anything in between.

### General flow

To be able to locate where things are failing we first need to know what the
expected steps are:

- [Deploying operators](#deploying-operators)
- [Propagating CRs](#propagating-crs)
- [Waiting for services](#waiting-for-services)
- [Deploying Glance](#deploying-glance)

##### Deploying operators

The expected result of running `make openstack` is to have the OpenStack
operators running in our OpenShift cluster.

The [Operator Lifecycle Manager (OLM)](https://olm.operatorframework.io/docs/)
is used to deploy the operators, and it is recommended to read its
documentation to understand it, but let's have a quick overview here.

When we are packaging our operator to be delivered through the OLM we create 3
container images, the operator itself, a bundle, and an index.

The bundle image contains the [`ClusterServiceVersion`
(CSV)](https://olm.operatorframework.io/docs/concepts/crds/clusterserviceversion/),
which we can think to be something like an RPM. It contains metadata about a
specific operator version as well as a template to be used by the OpenShift
Deployment operator to create our operator pods.

The index image holds a sqlite database with bundle definitions and it
runs a grpc service when executed that lets consumers query the operators.

The operator contains the service with the controllers for a number of CRDs.

So how is the `make openstack` command actually deploying our operators using
those images?

The first thing it does is create an [`OperatorGroup`](https://docs.openshift.com/container-platform/4.11/operators/understanding/olm/olm-understanding-operatorgroups.html) to
provide multitenant configuration selecting the target namespaces in which to
generate required RBAC access for its member Operators.

Then it creates a [`CatalogSource`](
https://olm.operatorframework.io/docs/concepts/crds/catalogsource/) to
represent a specific location that can be queried to discover and install
operators and their dependencies.  In our case this points to the index image,
that runs a grpc service as mentioned before.

This step is necessary because our operator we are installing is not present in
the default catalog included in OpenShift.

At this point OpenShift knows everything about our operators, but we still need
to tell it that we want to install that specific operator.  This is where the
[`Subscription`](
https://olm.operatorframework.io/docs/concepts/crds/subscription/) comes into
play.

A Subscription represents an intention to install an operator and the `make
openstack` command creates one for our operator and specifies our
`CatalogSource` as the source to find our operator.  This means that it will
install our custom operator even if we have an official operator already
released in the official operator catalog.

This newly created `Subscription` triggers the creation of the index pod so the
OLM can query the information, then an [`InstallPlan`](
https://docs.openshift.com/container-platform/4.11/rest_api/operatorhub_apis/installplan-operators-coreos-com-v1alpha1.html)
gets created to take care of installing the resources for the operator.

If the operators are not correctly deployed after running `make openstack`,
then we should look into the `InstallPlan` and check for errors.

```
oc describe InstallPlan | less
```

If there is no `InstallPlan` we have to check if the index pod is running:

```
oc get pod -l olm.catalogSource=openstack-operator-index
```

If it isn't, check for the `CatalogSource`:

```
oc describe catalogsource openstack-operator-index
```

##### Propagating CRs

When we run `make openstack_deploy` we are basically applying our
`OpenStackControlPlane` manifest, as defined in the `OPENSTACK_CR`
environmental variable, which is a CRD defined by the openstack-operator.

The openstack-operator has a controller watching for `OpenStackControlPlane`
resources, so when it sees a new one it starts working to reconcile it. In this
case that means propagating the `template` in the `glance` section into a new
`Glance` resource.

So after the `OpenStackControlPlane` resource we should be able to see the
`Glance` resource created by the openstack-operator, and this should contain
the same information present in the `template` section in the `glance` section
of our manifest.

```
oc get glance
```

If we don't see this resource, then we need to check first that the `enabled`
key inside the `glance` section is not set to `false` and then look at the
openstack-operator logs and search for reconciliation errors.

```
OPENSTACK_OPERATOR=`oc get pod -l control-plane=controller-manager -o custom-columns=POD:.metadata.name|grep openstack`
oc logs $OPENSTACK_OPERATOR
```

Something similar should happen for the keystone and cinder operators.

```
oc get keystoneapis
oc get cinder
```

##### Waiting for services

Now that we have a `Glance` resource it's the glance-operator's turn, that has
a specific controller waiting for these `Glance` resource, but before it starts
deploying the glance services it has to make sure that everything is in place
for the services to run correctly.

##### Deploying Glance

Now that everything is ready the `Glance` controller will request the creation
of the glance api service.

If the glance-operator is running successfully then it should generate those
resources based on the top level `Glance` resource and the information gathered
by the `mariadbdatabase`, and the generated `ConfigMap`s and `Secret`s.

If we don't see a specific resource kind it may be because 
there is a failure during it's creation.  In this case we should check the
glance-operator's log and look for the error:

```
GLANCE_OPERATOR=`oc get pod -l control-plane=controller-manager -o custom-columns=POD:.metadata.name|grep glance`
oc logs $GLANCE_OPERATOR
```

At this point we should see the pod for glance api service running, or at least
trying to run.  If they cannot run successfully we should `describe` the pod to
see what is failing.

It's important to know that these services use the [Kolla project](
https://wiki.openstack.org/wiki/Kolla) to prepare and start the service.

### OpenStack CRDs

The specific CRDs for the whole OpenStack effort can be listed after
successfully running `make openstack` with:

```
oc get crd -l operators.coreos.com/openstack-operator.openstack=
```

### Configuration generation

In the [waiting for services section](#waiting-for-services) we described the
creation of `ConfigMap`s and `Secret`s.  Those are basically the scripts and
default configuration from the `templates` directory, but then we need to add
to that default configuration the customizations from the user, such as the
backend configuration.

The final configuration is created by an [init container](
https://kubernetes.io/docs/concepts/workloads/pods/init-containers/) in each
service pod running `templates/glance/bin/init.sh` and using templates and
environmental variables.

The script generates files in `/var/lib/config-data/merged` before any other
container in the pod is started and then the directory is available in the
probe and service containers.

### Glance API service

Sometimes as a developer we need to make changes in configuration/policy files
or add more logs in actual code to see what went wrong in API calls. In
normal deployment it is not possible to do this on the fly and you need to
redeploy everything each time you make changes to either of the above. If
you launch the glance-api container(pod) in debug mode then you will be able
to do it on the fly without relaunching/recreating the container each
time. The following represents how you can launch the glance pod in debug
mode and make changes on the fly.

Enable `debug = True` for the container you want to launch in debug mode,
here we want to launch a user facing container (External) in debug mode.
```
(path to file: install_yamls/out/openstack/glance/cr/glance_v1beta1_glance.yaml)

glanceAPIExternal:
	debug:
  	service: true # Change it to true if it is false
	preserveJobs: false
	replicas: 1
```

Now rebuild the operator;
```
make generate && make manifests && make build
OPERATOR_TEMPLATES=$PWD/templates ./bin/manager
```

Once deployment is complete, you need to login to container;

```
oc exec -it glance-external-api-* bash
```

Verify that glance process is not running here;
```
$ ps aux | grep glance
root 	1635263  0.0  0.0   6392  2296 pts/3	S+   07:35   0:00 grep --color=auto glance
```

Now you can modify the code to add pdb or more logs by doing actual
changes to code base;
```
$ python3 -c "import glance;print(glance.__file__)"
/usr/lib/python3.9/site-packages/glance/__init__.py
```

Add debug/log statements or pdb at your desired location in
```
/usr/lib/python3.9/site-packages/<file_name>.py
```

Launch the glance service;
```
/usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start
```

Verify that glance process is running;
```
$ ps aux | grep glance
root   	13555  0.4  0.7 708732 123860 pts/1   S+   Nov15  35:33 /usr/bin/python3 /usr/bin/glance-api --config-dir /etc/glance/glance.conf.d
root   	13590  0.0  0.6 711036 99096 pts/1	S+   Nov15   0:03 /usr/bin/python3 /usr/bin/glance-api --config-dir /etc/glance/glance.conf.d
root   	13591  0.0  0.8 990744 135064 pts/1   S+   Nov15   0:04 /usr/bin/python3 /usr/bin/glance-api --config-dir /etc/glance/glance.conf.d
root   	13592  0.0  0.8 990232 134864 pts/1   S+   Nov15   0:03 /usr/bin/python3 /usr/bin/glance-api --config-dir /etc/glance/glance.conf.d
root 	1635263  0.0  0.0   6392  2296 pts/3	S+   07:35   0:00 grep --color=auto glance
```

Similar way you can modify configuration/policy files located in /etc/glance/* and kill
and start the service inside the container.

