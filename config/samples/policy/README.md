# Glance policy override

This directory includes an example of `policy.yaml` that can be injected to the
`GlanceAPI` service and overrides the default behavior. As the example shows,
a `policy.yaml` can be added to the Pod via `extraMounts`, which is valid
both locally and when the volume is provided via the global `OpenStackControlPlane`
CR.

## Create the ConfigMap where policy.yaml is stored

Before applying the `GlanceAPI` CR, a [policy.yaml](https://github.com/openstack-k8s-operators/glance-operator/tree/main/config/samples/policy/policy.yaml) file can be created and
customized according to the [upstream](https://docs.openstack.org/glance/latest/configuration/glance_policy.html)
documentation.
When the file is ready, create a `ConfigMap` with the following command:

```
oc -n <namespace> create configmap glance-policy --from-file=path/to/policy.yaml
```

This step can be skipped in the example provided, as the ConfigMap is automatically
created with the OpenStackControlPlane CR.

## Enable the oslo setting via customServiceConfig

As per the
[example](https://github.com/openstack-k8s-operators/glance-operator/tree/main/config/samples/glance_v1beta_glance_apply_policy.yaml),
when the `ConfigMap` is available, a `GlanceAPI` CR can be applied injecting
the configuration that will point to the new `policy.yaml` file.

```
spec:
  ...
  customServiceConfig: |
    [oslo_policy]
    policy_file=/etc/glance/policy.d/policy.yaml
...
```

The `ConfigMap` created in the previous section, can be mounted via `extraMounts`
and the mountpoint should match the `customServiceConfig` override definition:

```
...
  extraMounts:
    - name: v1
      region: r1
      extraVol:
        - propagation:
            - Glance
          extraVolType: Policy
          volumes:
            - name: glance-policy
              configMap:
                name: glance-policy
          mounts:
            - name: glance-policy
              mountPath: /etc/glance/policy.d
              readOnly: true
...
```

It is possible to create the `glance-policy` configMap along with the `OpenStackControlPlane` CR.
To deploy the `policy.yaml` sample provided, run the following command:

```bash
oc -n <namespace> kustomize --load-restrictor LoadRestrictionsNone ../policy | oc apply -f -
```

## Test Glance policies

Glance's public API calls may be restricted to certain sets of users using a
policy configuration file. This section explains exactly how to test APIs with
policies. Glance operator configures default policies in policy.yaml file which
will be provided as described in the previous section. The following represents
how you can test API(s) with glance policies.

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
