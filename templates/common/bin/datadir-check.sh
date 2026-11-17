#!/bin/bash
#
# Usage: datadir-check.sh <datadir> [<datadir>...]
#
# Verify the datadir of every enabled file store is usable before Glance
# starts. Runs as the glance user, so it sees the access glance-api will get.
#
# Nothing is repaired: the kubelet does not apply fsGroup to NFS volumes and a
# root_squash export can only be fixed on the server, by the administrator.
#
# Fail early: Glance evaluates store write access once at startup, and then
# stays healthy while rejecting every upload with StoreAddDisabled.

set -u

if [ "$#" -eq 0 ]; then
    echo "usage: $0 <datadir> [<datadir>...]" >&2
    exit 1
fi

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

check_datadir() {
    local datadir="$1"

    # A fresh PVC is empty and an export can be mounted above the datadir, so
    # create it as kolla did: it only works where glance can already write.
    if [ ! -d "${datadir}" ] && ! mkdir -p "${datadir}" 2>/dev/null; then
        echo "ERROR: ${datadir} does not exist and could not be created by ${WANT_UID}:${WANT_GID}." >&2
        echo "  Check the configured filesystem_store_datadir and the volume mounted for it." >&2
        echo "  ls -ld (parent): $(ls -ld "$(dirname "${datadir}")" 2>&1)" >&2
        hint
        return 1
    fi

    # Uploads need write and search access, downloads need read access. On NFS
    # these are resolved by an ACCESS RPC, so squashing and ro exports are covered.
    if [ ! -r "${datadir}" ] || [ ! -w "${datadir}" ] || [ ! -x "${datadir}" ]; then
        echo "ERROR: ${datadir} is not usable by the glance user (${WANT_UID}:${WANT_GID})." >&2
        echo "  ls -ld: $(ls -ld "${datadir}" 2>&1)" >&2
        hint
        return 1
    fi

    echo "${datadir} is usable by ${WANT_UID}:${WANT_GID} ($(ls -ld "${datadir}"))"
    # A datadir with no volume mounted on it silently lands on the local PVC,
    # so report what backs it: the store is only shared if this says nfs.
    echo "  backed by: $(df -PT "${datadir}" 2>&1 | tail -1)"
}

# Check them all: a multi store deployment should not have to fix one export,
# restart, and discover the next one is broken too.
rc=0
for datadir in "$@"; do
    check_datadir "${datadir}" || rc=1
done
exit "${rc}"
