# Glance policy override

This directory includes an example of `policy.yaml` that can be injected to the
`GlanceAPI` service and overrides the default behavior. As the example shows,
a `policy.yaml` can be added to the Pod via `extraMounts`, which is valid
even when the volume is provided via the `OpenStackControlPlane` CR.

## Create the ConfigMap where policy.yaml is stored

Before applying the `GlanceAPI` CR, a [policy.yaml](https://github.com/openstack-k8s-operators/glance-operator/tree/main/config/samples/policy/policy.yaml) file can be created and
customized according to the [upstream](https://docs.openstack.org/glance/latest/configuration/glance_policy.html)
documentation.
When the file is ready, create a `ConfigMap` with the following command:

```
oc -n <namespace> create configmap glance-policy --from-file=path/to/policy.yaml
```

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
