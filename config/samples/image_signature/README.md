# Image Signature Verification

Glance has the ability to perform image validation using a digital signature and
asymmetric cryptography.

A set of properties should be defined and specified for the target image.
Glance will validate the uploaded image data against these properties before
storing it in the configured backend. If validation fails, the upload is stopped
and the image is deleted.
The secret used to perform the signature verification must be stored in `Barbican`.
`Barbican` is deployed by default in a generic `OpenStackControlPlane`, and `Glance`
is able to interact with it as per the following configuration already provided
by the `glance-operator`.

```
[key_manager]
backend = barbican

[barbican]
auth_endpoint={{ .KeystoneInternalURL }}
barbican_endpoint_type=internal
```

As per the snippet above, `Glance` is able to reach `Barbican` and interact with
it through the internal endpoint provided by `Keystone`.
The image properties might be used by other services (e.g., `Nova`) to perform
data verification when the image is downloaded.

You can find more details about this feature in the [upstream](https://docs.openstack.org/glance/latest/user/signature.html)
documentation.


## Example: Create a signed image

The following steps are used to create a signed image that will be uploaded in
`Glance`.


- Create the keys and the certificate

```bash
$ openssl genrsa -out private_key.pem 1024

Generating RSA private key, 1024 bit long modulus
...............................................++++++
..++++++

```

```bash
$ openssl rsa -pubout -in private_key.pem -out public_key.pem
writing RSA key
```


```bash
$ openssl req -new -key private_key.pem -out cert_request.csr
```

You are asked to enter information that will be incorporated in your certificate
request.


```bash
$ openssl x509 -req -days 14 -in cert_request.csr -signkey private_key.pem -out new_cert.crt
Signature ok
```

- Upload the certificate in `Barbican` that will be used to store the secret:

```bash
$ openstack secret store --name test --algorithm RSA --secret-type certificate \
  --payload-content-type "application/octet-stream" \
  --payload-content-encoding base64 \
  --payload "$(base64 new_cert.crt)"

+---------------+-----------------------------------------------------------------------+
| Field         | Value                                                                 |
+---------------+-----------------------------------------------------------------------+
| Secret href   | http://127.0.0.1:9311/v1/secrets/cd7cc675-e573-419c-8fff-33a72734a243 |
+---------------+-----------------------------------------------------------------------+
```

Get the `cert_uuid` from the secret generated in the previous step:

```bash
$ cert_uuid=cd7cc675-e573-419c-8fff-33a72734a243
```

- Get an image and create the signature.

```bash
$ echo This is a dodgy image > myimage

$ openssl dgst -sha512 -sign private_key.pem -sigopt rsa_padding_mode:pss -out myimage.signature myimage

$ base64 -w 0 myimage.signature > myimage.signature.b64

$ image_signature=$(cat myimage.signature.b64)

$ cat myimage.signature.b64
  KFYrU1XIFXC6RyYO9AhYB4VN1vnm+ImZyEuYLHXzgebs7sJGIdoLfb4ELnfIHI5Ijm/H/zUiY0BQGMbrzzoFUQQkOacu7oU24CxDLP3XrjI0nE9d1qmCH6m81/eVeEhyp/NCKi7NSYaadp4jCMI1gLyNZMbQCBKWeT784/4jPgs=
```


- Create image with valid signature properties

```bash
$ glance --os-auth-url {{ KeystoneAuthUrl }} \
         --os-project-name {{ project }} --os-username {{ user }} --os-password {{ pwd }} \
         --os-user-domain-name default --os-project-domain-name default \
         image-create --name mySignedImage --container-format bare --disk-format qcow2 \
         --property img_signature="$image_signature" --property img_signature_certificate_uuid="$cert_uuid" \
         --property img_signature_hash_method='SHA-512' --property img_signature_key_type='RSA-PSS' < myimage
```

### Note:

Replace all the variables of the `image-create` command with the data provided
by the `clouds.yaml` file. The `clouds.yaml` file can be either retrieved by:

```bash
oc get cm openstack-config -o json | jq -r '.data["clouds.yaml"]'
```

or, if the `openstackclient` `Pod` is used to perform such commands, it can be
found in the `$HOME/.config/openstack/` path.

The password passed to the `--os-password` option can be retrieved by querying the `OpenStack` secret.
```bash
oc get secrets osp-secret -o jsonpath="{ .data.AdminPassword }" | base64 -d
```

- osp-secret: the secret used to store the passwords associated to the deployed
  ctlplane


## Known Limitations

- The signature verification is based on `openSSL` `_rsa_sig_verify()`, which
currently fails if the key length is different than 1024, raising an
`InvalidSignature` exception on the Glance front.
This issue represent a regression connected to the `openSSL` 3.0.7 library, while
it works properly with key length 2048 with openSSL 3.0.2.

- For the same reason of the previous item, the image signature verification fails
  if sha256 is used to build the digest.
