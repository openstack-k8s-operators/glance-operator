# OPENSTACKCLIENT

The `openstackclient` is used to interact with an existing (even minimal)
control plane, and it can be used by `kuttl` tests to execute scripts that are
supposed to verify that a given functionality works as expected.
It assumes that `keystone` is available and the ConfigMap containing cloud
credentials already exist and can be mounted to the target `Pod`.

## Deploy the openstackclient via kustomize

It is possible to inject scripts providing the right mounts to the
`openstackclient` via [kustomize](https://kustomize.io).
In general, there are two allowed/supported scenarios:
- inject a particular script to test a feature and verify there are no regressions
- inject the script directory that contains the whole set of scripts: an external
  test suite will select the right one as needed

In case you want to inject a particular script, add the following snippet in the
kustomization.yaml file:

```
- op: add
  path: /spec/volumes/0
  value:
   configMap:
     name: openstack-scripts
     defaultMode: 755
   name: openstack-scripts
- op: add
  path: /spec/containers/0/volumeMounts/0
  value:
    mountPath: /home/cloud-admin/import-image-container.sh
    name: distributed-image-import
    subPath: import-image-container.sh
```

The snippet above assumes the goal is to inject a single script only called
`import-image-container.sh` to the `cloud-admin` `$HOME` directory, and it will
be available under the defined `subPath`. If the purpose is to test multiple
features, it is possible to inject the entire `scripts` directory and select
the script that should be executed from an external test suite:

```
- op: add
  path: /spec/containers/0/volumeMounts/0
  value:
   mountPath: /home/cloud-admin/scripts
   name: openstack-scripts
- op: add
  path: /spec/volumes/0
  value:
    configMap:
      defaultMode: 0755
      name: openstack-scripts
    name: openstack-scripts
```

Without a `subPath`, we can mount all the scripts provided by the `ConfigMap`
within a `scripts` directory.
This represents the default approach for kuttl tests.
All the potential changes (`add/remove/replace` fields) are managed by
`kustomize`, and the `openstackclient.yaml` definition should not be changed.
Once everything is ready, the `openstackclient` `Pod` can be deployed with the
following command:

```bash
oc -n <namespace> kustomize --load-restrictor LoadRestrictionsNone $pwd/openstackclient | oc apply -f -
```

**Note:**

All the theory describe above assumes the `scripts` directory is used as source
to build a `ConfigMap` that is then mounted to the `Pod`. **Before** going
through the described steps, make sure a `ConfigMap` containing the scripts
directory exists, or at least each script entry is mentioned in kustomization.yaml.
The mentioned configmap can be manually built with the following command:

```bash
oc -n <namespace> cm openstack-scripts create --from-file=<path/to/scripts/directory>
```

However, a `ConfigMapGenerator` has been added to the `kustomization.yaml` script.
It might be possible to use `kustomize` to include additional scripts:

```bash
kustomize edit add configmap openstack-scripts --from-file=script/<script_name>
```

but it might not work is the `script` directory is defined in a path outside the
kustomization.yaml workdir. In this case just make sure to add the script entry
within the kustomization.yaml file:

```bash
configMapGenerator:
- files:
  - ../import_plugins/distributed_image_import/import-image-container.sh
  - ../image_signature/sign_glance_image.sh
  - ...
  - ...
  name: openstack-scripts
```

## Include the openstackclient in a kuttl test

To define a new kuttl `TestStep` where a script is executed through the `Command`
kuttl object, it is possible to use the `openstackclient` to perform a feature
verification against a particular component.
The steps to define a new kuttl test step are:
1. Create the bash script that contains all the steps required to test a given
   functionality; kuttl makes no assumption about the content of the script, so
   it should take care about the logic of terminating with the right exit code
   to determine `PASS` or `FAIL`
2. Add the script created at step 1 into `config/samples/<>/`, or `hack/scripts`
   directory of the service operator if it's not associated with any particular
   feature
3. Update the `ConfigMapGenerator` entry in `kustomization.yaml` to include the
   script added in `hack/scripts` or `config/samples/<>/` directory at step 2.
   The `ConfigMap` can be updated with the following command:

   ```bash
   pushd $PWD/openstackclient
   kustomize edit add configmap openstack-scripts --from-file=script/<script_name>
   popd
   ```
   in case the scripts folder is local to the `openstackclient` sample
4. Add the `openstackclient` to the newly defined kuttl test (typically to a
   `00-deploy.yaml`that represents the deployment step (Note that
   `00-deploy.yaml` might not be the same for all the tests and might have a
   different meaning):

   ```bash
   oc -n <namespace> kustomize --load-restrictor LoadRestrictionsNone ../../../openstackclient | oc -n <namespace> apply -f -
   ```
   The command above creates the `openstack-scripts` `ConfigMap`, and, at the
   same time, deploys an `openstackclient` `Pod`
5. Add, to the last kuttl step (which is generally suffixed with `-cleanup`),
    the following line:
   ```bash
   oc -n <namespace> kustomize --load-restrictor LoadRestrictionsNone ../../../openstackclient | oc -n <namespace> delete -f -
   ```
   The above ensures the resource is terminated when the environment is cleaned
   up and the test returned its status to the end user.
