#!/bin/bash
#
# datadir-setup.sh - make the Glance file backend image store directory
# writable by the glance user, and refuse to let the pod start if it cannot.
# Runs as uid 0 in the datadir-setup init container, before glance-api starts.
#
# Usage: datadir-setup.sh <datadir> <want-uid> <want-gid>
#
# The kubelet does not apply fsGroup ownership management to NFS volumes -- the
# NFS volume plugin reports Managed=false, so SetVolumeOwnership is never
# called -- which means an export created as root:root 0755 is not writable by
# the glance user. A PVC is block backed and does get fsGroup applied, so this
# is a no-op there.
#
# The chown can only help on an export mounted no_root_squash, because it is
# issued as uid 0 and root_squash maps that to the anonymous user. A
# root_squash export whose ownership was prepared correctly server-side is
# already fine and must not be touched, so the writability check runs first and
# skips the chown entirely.
#
# Failing stops the pod: glance_store evaluates WRITE_ACCESS once, in
# configure_add(), and the filesystem driver does not override
# update_capabilities(), so an API that starts against an unwritable datadir
# stays unable to accept uploads for the lifetime of the process while still
# reporting healthy, rejecting every upload with StoreAddDisabled.

set -u

DATADIR="${1:?usage: $0 <datadir> <want-uid> <want-gid>}"
WANT_UID="${2:?usage: $0 <datadir> <want-uid> <want-gid>}"
WANT_GID="${3:?usage: $0 <datadir> <want-uid> <want-gid>}"

# Whether uid WANT_UID could create entries in the directory, which needs both
# write and execute. Evaluated from the mode rather than with test -w, because
# this container runs as root and root bypasses the permission check.
glance_can_write() {
    local st uid gid mode u g o
    st="$(stat -c '%u %g %a' "$1")" || return 1
    uid="${st%% *}"; st="${st#* }"
    gid="${st%% *}"; mode="${st##* }"
    # keep the low three octal digits, dropping any setuid/setgid/sticky prefix
    mode="00${mode}"; mode="${mode: -3}"
    u="${mode:0:1}"; g="${mode:1:1}"; o="${mode:2:1}"
    [ "${uid}" = "${WANT_UID}" ] && [ $(( u & 3 )) -eq 3 ] && return 0
    [ "${gid}" = "${WANT_GID}" ] && [ $(( g & 3 )) -eq 3 ] && return 0
    [ $(( o & 3 )) -eq 3 ] && return 0
    return 1
}

# Last resort before failing: ask the kernel as the glance user. This catches a
# directory made writable by an ACL, which glance_can_write cannot see. Only
# ever consulted to pass, never to fail, so a missing or uncooperative su can
# never block a working deployment.
glance_can_write_acl() {
    command -v su >/dev/null 2>&1 || return 1
    su -s /bin/sh glance -c 'test -w "$1"' sh "$1" >/dev/null 2>&1
}

fail() {
    echo "ERROR: ${DATADIR} is not writable by ${WANT_UID}:${WANT_GID}." >&2
    echo "  ls -ld: $(ls -ld "${DATADIR}" 2>&1)" >&2
    echo "  The kubelet does not apply fsGroup to NFS volumes, so the export must be" >&2
    echo "  owned by ${WANT_UID}:${WANT_GID}, or group writable by ${WANT_GID}, on the" >&2
    echo "  NFS server. Under root_squash this cannot be fixed from the client." >&2
    exit 1
}

if [ ! -d "${DATADIR}" ]; then
    echo "Creating ${DATADIR}"
    if ! mkdir -p "${DATADIR}"; then
        echo "ERROR: could not create ${DATADIR}." >&2
        echo "  Under root_squash our uid 0 is mapped to the anonymous user; create" >&2
        echo "  the directory owned by ${WANT_UID}:${WANT_GID} on the NFS server." >&2
        exit 1
    fi
fi

if glance_can_write "${DATADIR}"; then
    echo "${DATADIR} is already writable by ${WANT_UID}:${WANT_GID} ($(stat -c '%U:%G %A' "${DATADIR}")), nothing to do"
    exit 0
fi

echo "Setting ownership of ${DATADIR} from $(stat -c '%u:%g' "${DATADIR}") to ${WANT_UID}:${WANT_GID}"
# Deliberately non-recursive. On a populated image store a recursive chown over
# NFS is expensive, and images written by a correctly configured glance are
# already owned by ${WANT_UID}.
if ! chown "${WANT_UID}:${WANT_GID}" "${DATADIR}"; then
    echo "WARNING: chown of ${DATADIR} failed; most likely a root_squash export." >&2
    glance_can_write_acl "${DATADIR}" || fail
    echo "${DATADIR} is writable by ${WANT_UID} via an ACL, continuing"
    exit 0
fi

# chown can report success while the server squashed it to the anonymous uid,
# so re-check rather than trusting the exit status.
if ! glance_can_write "${DATADIR}"; then
    glance_can_write_acl "${DATADIR}" || fail
fi

echo "${DATADIR} is now owned by $(stat -c '%u:%g' "${DATADIR}")"
