# Controlling resources for web-download import method

You can limit the sources of web-import image downloads by
specifying URI `blocklists` and `allowlists` in the glance
configuration file.

 You can allow or block image source URIs at three levels:

- scheme (allowed_schemes, disallowed_schemes)
- host (allowed_hosts, disallowed_hosts)
- port (allowed_ports, disallowed_ports)

If you specify both `allowlist` and `blocklist` at any level, the
`allowlist` is honored and the `blocklist` is ignored.

The Image service (glance) applies the following decision logic to
validate image source URIs:

1. The scheme is checked.
   1. Missing scheme: reject
   2. If there is an `allowlist`, and the scheme is not present in the `allowlist`: reject.
      Otherwise, skip iii. and continue on to 2.
   3. If there is a `blocklist`, and the scheme is present in the `blocklist`: reject.
2. The host name is checked.
   1. Missing host name: reject
   2. If there is an `allowlist`, and the host name is not present in the `allowlist`: reject.
      Otherwise, skip iii. and continue on to 3.
   3. If there is a `blocklist`, and the host name is present in the `blocklist`: reject.
3. If there is a port in the URI, the port is checked.
   1. If there is a `allowlist`, and the port is not present in the `allowlist`: reject.
      Otherwise, skip ii. and continue on to 4.
   2. If there is a `blocklist`, and the port is present in the `blocklist`: reject.
4.  The URI is accepted as valid.

If you allow a scheme, either by adding it to an `allowlist` or
by not adding it to a `blocklist`, any URI that uses the default
port for that scheme by not including a port in the URI is allowed.
If it does include a port in the URI, the URI is validated according
to the default decision logic.

## Image import `allowlist` example

In this example, we are assuming to use ftp server for image
upload. The default port for FTP is 21.

Because `ftp` is in the list for `allowed_schemes`, this URL to the image
resource is allowed: ftp://example.org/some/resource.

However, because 21 is not in the list for `allowed_ports`, this URL to the
same image resource is rejected: ftp://example.org:21/some/resource.

```
  glance:
    template:
      customServiceConfig: |
        [DEFAULT]
        allowed_schemes = [http,https,ftp]
        disallowed_schemes = []
        allowed_hosts = []
        disallowed_hosts = []
        allowed_ports = [80,443]
        disallowed_ports = []
      glanceAPIs:
        ...
```

## Default image import `blocklist` and `allowlist` settings

Below are the default options provided for `allowlist` and `blcoklist`.

- allowed_schemes - [http, https]
- disallowed_schemes - []
- allowed_hosts - []
- disallowed_hosts - []
- allowed_ports - [80, 443]
- disallowed_ports - []

If you use the defaults, end users can access URIs by using only the
`http` or `https` scheme. The only ports that users can specify are
`80` and `443`. Users do not have to specify a port, but if they do,
it must be either `80` or `443`.

## Configuring resources for web-download import method

Assuming you are using `install_yamls` and you already have `crc` running, you
can use the provided `web_download` example with:


```
$ cd install_yamls
$ make crc_storage openstack
$ oc kustomize ../glance-operator/config/samples/web_download > ~/openstack-deployment.yaml
$ export OPENSTACK_CR=`realpath ~/openstack-deployment.yaml`
$ make openstack_deploy
```

If we already have a deployment working we can always use `oc kustomize ../web_download | oc apply -f -`
from this directory to make the changes.

You can find more about disk-formats configuration options in the
[upstream](https://docs.openstack.org/glance/latest/admin/interoperable-image-import.html#configuring-the-web-download-method) documentation.
