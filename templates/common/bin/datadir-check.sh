#!/bin/bash
#
# Verify the file-backend datadir is usable before Glance starts.
# Runs as the glance user in the datadir-check init container, so the checks
# below observe exactly the access glance-api will get at runtime.
#
# Usage: datadir-check.sh <datadir>
#
# Nothing is repaired here: the kubelet does not apply fsGroup ownership to NFS
# volumes, and a root_squash export cannot be fixed from the client anyway.
# Ownership and permissions are the storage administrator's responsibility.
#
# Fail early: Glance evaluates store write access once at startup. An unwritable
# store leaves the API healthy but rejecting every upload with StoreAddDisabled,
# which is much harder to diagnose than a pod that refuses to start.

set -u

DATADIR="${1:?usage: $0 <datadir>}"
WANT_UID="$(id -u)"
WANT_GID="$(id -g)"

# What the administrator has to fix, printed with every failure.
hint() {
    echo "  id: $(id 2>&1)" >&2
    echo "  The image store must be owned by ${WANT_UID}:${WANT_GID}, or be group" >&2
    echo "  owned by ${WANT_GID} and group rwx. The kubelet does not apply fsGroup" >&2
    echo "  to NFS volumes, so for an NFS backend this has to be set on the server" >&2
    echo "  side, on the exported directory." >&2
}

fail() {
    echo "ERROR: ${DATADIR} is not usable by the glance user (${WANT_UID}:${WANT_GID})." >&2
    echo "  ls -ld: $(ls -ld "${DATADIR}" 2>&1)" >&2
    hint
    exit 1
}

# A freshly provisioned PVC is empty, and an NFS export may be mounted above the
# datadir rather than at it, so the directory is not guaranteed to exist.
# Creating it is what kolla used to do, and it only succeeds when the parent is
# already writable by glance, which is the same permission requirement.
if [ ! -d "${DATADIR}" ] && ! mkdir -p "${DATADIR}" 2>/dev/null; then
    echo "ERROR: ${DATADIR} does not exist and could not be created by ${WANT_UID}:${WANT_GID}." >&2
    echo "  Check the configured filesystem_store_datadir and the volume mounted for it." >&2
    echo "  ls -ld (parent): $(ls -ld "$(dirname "${DATADIR}")" 2>&1)" >&2
    hint
    exit 1
fi

# Uploads need write and search access, downloads need read access. On NFS
# these are resolved by an ACCESS RPC, so squashing and ro exports are covered.
if [ ! -r "${DATADIR}" ] || [ ! -w "${DATADIR}" ] || [ ! -x "${DATADIR}" ]; then
    fail
fi

echo "${DATADIR} is usable by ${WANT_UID}:${WANT_GID} ($(ls -ld "${DATADIR}"))"
