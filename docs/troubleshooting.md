# Troubleshooting guide

Troubleshooting the Glance Service involves running the `oc debug` command as
described in the [openstack-k8s-operators doc](https://github.com/openstack-k8s-operators/docs/blob/main/troubleshooting.md).
It is possible to choose which container should be run with the debug command
simply passing the `-c <container` flag. The available containers are defined
and described in the [design document](https://github.com/openstack-k8s-operators/glance-operator/blob/main/docs/design-decisions.md).
As an example, let's suppose that the goal is to perform some troubleshooting
against the `GlanceAPI service` (not the operator).

Run the following command to run an existing glanceAPI `Pod` in debug mode:

```bash
oc debug pod/glance-default-external-api-0 --keep-labels=true
```

It results in a duplicated pod, where the designed command is not run, and it
returns an interactive shell that can be used to start the troubleshooting.

```bash
Starting pod/glance-default-external-api-0-debug-fz4hz, command was: /bin/bash -c exec /usr/sbin/httpd -DFOREGROUND
Pod IP: --
If you don't see a command prompt, try pressing enter.
sh-5.1#
```

It is possible to list the Pod using the `service=glance` label:

```bash
[stack@osp-storage-05 install_yamlsss]$ oc get pods -l service=glance -w
NAME                                        READY   STATUS
glance-default-external-api-0               0/3     CrashLoopBackOff
...
glance-default-external-api-0-debug-fz4hz   0/3     Pending
glance-default-external-api-0-debug-fz4hz   1/3     Running
glance-default-external-api-0-debug-fz4hz   1/3     NotReady
```

Config files are already mounted directly at their final locations (see
`internal/glance/volumes.go`) -- there's no kolla-style copy step to run
anymore. `/etc/glance/glance.conf.d/01-config.conf` (the runtime-only
`worker_self_reference_url` snippet, only relevant when `GLANCE_DOMAIN` is
set for distributed image import) is the one file not pre-populated from a
Secret; write it by hand if you need it for troubleshooting:

```bash
sh-5.1# if [ -n "$GLANCE_DOMAIN" ]; then cat > /etc/glance/glance.conf.d/01-config.conf <<EOF
[DEFAULT]
worker_self_reference_url=${URISCHEME,,}://$(hostname).${GLANCE_DOMAIN}:${GLANCE_PORT}
EOF
fi
```

Verify the configuration files before starting the service:


```bash
sh-5.1# ls -l /etc/glance/glance.conf.d
00-config.conf  02-config.conf  03-config.conf
sh-5.1# ls -l /etc/httpd/conf.d
10-glance-wsgi.conf  autoindex.conf  README  ssl.conf  userdir.conf  welcome.conf
sh-5.1# ls -l /etc/httpd/conf
httpd.conf  ...
```

Run the glance-api in background:

```bash
sh-5.1# glance-api --config-dir /etc/glance/glance.conf.d &
```

The process is running on localhost, and we can point our client directly to
the temporary API instead of relying on keystone discovery.
Setup the `glance client` that will be useful to run any command against the
temporary `glanceAPI` that we just deployed for troubleshooting purposes.

```bash
sh-5.1# alias glance="glance \
    --os-auth-url ${AUTH_URL} \
    --os-project-name ${USER} \
    --os-username ${USER} \
    --os-password ${PASSWORD} \
    --os-user-domain-name default \
    --os-project-domain-name default "
```

Check the API works as expected running an `image-list` command:

```bash
sh-5.1# glance --os-image-url "http://localhost:9293" image-list

+----+------+
| ID | Name |
+----+------+
+----+------+

sh-5.1# glance --os-image-url "http://localhost:9293" image-create

+------------------+--------------------------------------+
| Property         | Value                                |
+------------------+--------------------------------------+
| checksum         | None                                 |
| container_format | None                                 |
| created_at       | 2024-02-28T11:47:44Z                 |
| disk_format      | None                                 |
| id               | 89ef001d-9834-46bc-a135-54e9f9bd42ae |
| min_disk         | 0                                    |
| min_ram          | 0                                    |
| name             | None                                 |
| os_hash_algo     | None                                 |
| os_hash_value    | None                                 |
| os_hidden        | False                                |
| owner            | e29caea6d7244484b9e04c30b289f281     |
| protected        | False                                |
| size             | None                                 |
| status           | queued                               |
| tags             | []                                   |
| updated_at       | 2024-02-28T11:47:44Z                 |
| virtual_size     | Not available                        |
| visibility       | shared                               |
+------------------+--------------------------------------+

sh-5.1# glance --os-image-url "http://localhost:9293" image-list

+--------------------------------------+------+
| ID                                   | Name |
+--------------------------------------+------+
| 89ef001d-9834-46bc-a135-54e9f9bd42ae |      |
+--------------------------------------+------+
sh-5.1# glance --os-image-url "http://localhost:9293" image-delete 89ef001d-9834-46bc-a135-54e9f9bd42ae
sh-5.1# glance --os-image-url "http://localhost:9293" image-list

+----+------+
| ID | Name |
+----+------+
+----+------+
```

**Note**:

It is possible to point to port `9292` provided that `httpd` is running.
`httpd.conf` and the wsgi/proxypass vhost config are already mounted at
their final locations (`/etc/httpd/conf/httpd.conf`, `/etc/httpd/conf.d/`),
so `httpd` can be started directly, no copy step required:


```bash
sh-5.1# httpd -DFOREGROUND &
```

To automate part of the steps described in this guide, a
[script](https://github.com/openstack-k8s-operators/glance-operator/blob/main/hack/troubleshoot_api_setup.sh)
is provided.
It can be copied to the duplicated `Pod` and then executed to bootstrap a `GlanceAPI`.
It results in a running `glance-api` and `httpd` process, and once a glance
client is generated, it is possible for a human administrator to start
interacting with the service.
To execute the script run the following commands:

```bash
curl -O https://raw.githubusercontent.com/openstack-k8s-operators/glance-operator/main/hack/troubleshoot_api_setup.sh
oc cp -c glance-api troubleshoot_api_setup.sh glance-default-external-api-0-debug:/
oc rsh -c glance-api glance-default-external-api-0-debug
./troubleshoot_api_setup.sh
```

where `glance-default-external-api-0-debug` is the `Pod` generated by the `oc debug`
command.
