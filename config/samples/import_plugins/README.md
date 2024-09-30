# Glance import plugins samples

This directory includes a set of Glance import plugins samples that can be
used to enable specific or multiple glance import plugin(s) in the deployment.

These samples are not meant to serve as deployment recommendations, just as
working examples to serve as reference.

Currently available sample plugins are:

- Image Conversion
- Inject Image Metadata
- Image Decompression

## Enable Image Conversion plugin

Whenever Glance is configured to use Ceph as a backend, operators want to
optimize the backend capabilities by ensuring that all images will be in raw
format while not putting the burden of converting the images to their end users.
When Ceph is detected as a backend for Glance, the glance-operator injects and
enables the image-conversion plugin.
No action is required by the human operator, and this feature is enabled by
default. See [Ceph backend](https://github.com/openstack-k8s-operators/glance-operator/tree/main/config/samples/backends#ceph-example)
for additional details.
You can find more abut plugin configuration options
in [upstream](https://docs.openstack.org/glance/latest/admin/interoperable-image-import.html#the-image-conversion)
documentation.

## Enable Inject metadata plugin

One use case for this plugin is a situation where an operator wants to put
specific metadata on images imported by end users so that virtual machines
booted from these images will be located on specific compute nodes. Since
it’s unlikely that an end user (the image owner) will know the appropriate
properties or values, an operator may use this plugin to inject the
properties automatically upon image import.

Operator/Deployer can use the ‘customServiceConfig‘ section to enable
[`inject_image_metadata`](inject_metadata/inject_metadata.yaml) plugin and
specify plugin configuration options which will be copied to glance
configuration file.

You can find more abut plugin configuration options
in [upstream](https://docs.openstack.org/glance/latest/admin/interoperable-image-import.html#the-image-property-injection-plugin)
documentation.

## Image Decompression plugin

This plugin implements automated image decompression for Interoperable Image
Import. One use case for this plugin would be environments where the user or
operator wants to use the 'web-download' method and the image provider
supplies only compressed images.

Operator/Deployer can use the ‘customServiceConfig‘ section to enable
[`image_decompression`](image_decompression/image_decompression.yaml) plugin
and specify plugin configuration options which will be copied to glance
configuration file.

The plugin will not decompress images whose container_format is set to
'compressed' to maintain the original intent of the image creator. If Image
Conversion is used together, decompression must happen first, this is ensured
by ordering the plugins.
Make sure to properly plan storage for the Glance Pod when this feature is
enabled, especially if is enabled in combination with other image plugins.

You can find more information about storage planning in the design assumptions
[section](../../../docs/dev/design-decisions.md).

You can find more about plugin configuration options
in [upstream](https://docs.openstack.org/glance/latest/admin/interoperable-image-import.html#the-image-decompression)
documentation.
