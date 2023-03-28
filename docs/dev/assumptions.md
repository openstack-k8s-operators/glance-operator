# Assumptions

The different articles describing how to run custom glance-operator code make
some assumptions, please adjust the steps in those cases where your system
doesn't match them:

- You have an OpenShift cluster running and you have logged in it with `oc`, so
  there are valid credentials in the system.

- OpenStack operator repositories that are referenced are located directly in a
  directory within the home directory following the GitHub repository name. In
  the glance-operator case this will be `~/glance-operator`.

- The `install_yamls` repository is in `~/install_yamls`.

- Local repositories will have 2 remotes defined `origin` for our fork and
  `upstream` for the `openstack-k8s-operators` repository.
