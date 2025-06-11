# Glance Notifications

The OpenStack Image Service (Glance), like other core OpenStack services, can
generate notifications for various events that occur throughout the image
lifecycle. These notifications provide valuable telemetry data for auditing,
troubleshooting, monitoring operations, and integration with other services
such as Ceilometer for metrics collection and processing.
While Glance operates independently without requiring RabbitMQ for core
functionality, RabbitMQ serves as the message broker specifically for
notification delivery. When a `transportURL` is configured, the
`glance-operator` automatically updates the `oslo_notifications` section in the
`00-config.conf` file. This configuration change switches the notification
driver from `noop` (the default strategy that produces no notifications) to
`messagingv2`, which enables notification delivery to the configured message
queue.

For comprehensive information about notification types and their available
content/payload structures, refer to the [upstream
documentation](https://docs.openstack.org/glance/latest/admin/notifications.html).

## Enabling Notifications

The `glance-operator` exposes an API parameter called `notificationBusInstance`
that specifies the RabbitMQ instance name to use for requesting a
`TransportURL`. This URL is then configured in the Glance service through a
generated `Secret` object.

### Configuration via OpenStackControlPlane

Edit the `OpenStackControlPlane` specification and add the `notificationBusInstance` parameter to the `Glance` template section:

```yaml
...
spec:
  ...
  glance:
    template:
      notificationBusInstance: rabbitmq
  ...
```

Alternatively, you can patch the `OpenStackControlPlane` directly using the
following command:

```bash
OSCP=$(oc get oscp -o custom-columns=NAME:.metadata.name --no-headers)
oc -n openstack patch oscp $OSCP --type json -p='[{"op": "add", "path": "/spec/glance/template/notificationBusInstance", "value": "rabbitmq"}]'
```

### Verification

After applying the configuration, verify that the notification settings have
been properly updated by checking the configuration file:

```bash
$ oc rsh -c glance-httpd glance-default-external-api-0 grep "oslo_messaging_notifications" /etc/glance/glance.conf.d/00-config.conf -A 2

[oslo_messaging_notifications]
driver=messagingv2
transport_url = rabbit://<user>:<pwd>@rabbitmq.openstack.svc:5671/?ssl=1
```

## Disabling Notifications

When notifications are enabled, you can disable them by reverting the driver
back to `noop`. This is accomplished by removing the `notificationBusInstance`
parameter from the `Glance` template section in the `OpenStackControlPlane`.
This action triggers a reconciliation loop that updates the `GlanceAPI`
configuration and initiates a rolling update of the pods.

### Disabling via oc client

Use the following patch command to remove the notification configuration:

```bash
oc patch openstackcontrolplane openstack-galera -n openstack --type json -p='[{"op": "remove", "path": "/spec/glance/template/notificationBusInstance"}]'
```

This operation will trigger the reconciliation process, updating the
`GlanceAPI` configuration and causing a pod rollout with the new settings.

## Example Deployment

This example demonstrates how to deploy OpenStack with Glance notifications
enabled using `install_yamls`. The procedure assumes you have `crc` (CodeReady
Containers) running and ready.


### Deployment Steps

```bash
$ cd install_yamls
$ make ceph TIMEOUT=90
$ make crc_storage openstack openstack_init
$ oc kustomize ../glance-operator/config/samples/notifications > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

### Updating Existing Deployment

If you already have a working OpenStack deployment, you can apply the
notification configuration changes directly:

```bash
$ oc kustomize ../notifications | oc apply -f -
```

Execute this command from the appropriate directory containing your
notification configuration files.
