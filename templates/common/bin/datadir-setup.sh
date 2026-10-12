#!/bin/bash
#
# Ensure the existing file-backend datadir is writable before Glance starts.
# Runs as root in the datadir-setup init container.
#
# Usage: datadir-setup.sh <datadir> <want-uid> <want-gid>
#
# Kubelet does not apply fsGroup ownership to NFS volumes. Skip chown if the
# datadir is already writable; root_squash exports need server-side ownership
# setup because client root is mapped to the anonymous user.
#
# Fail early: Glance checks upload access once at startup. An unwritable store
# can leave the API healthy but rejecting uploads with StoreAddDisabled.

set -u

DATADIR="${1:?usage: $0 <datadir> <want-uid> <want-gid>}"
WANT_UID="${2:?usage: $0 <datadir> <want-uid> <want-gid>}"
WANT_GID="${3:?usage: $0 <datadir> <want-uid> <want-gid>}"

# Check write and search bits for WANT_UID:WANT_GID; test -w as root would
# bypass ordinary permissions. This check does not interpret ACLs.
glance_can_write() {
    local out uid gid mode
    out="$(stat -c '%u %g %a' "$1")" || return 1
    read -r uid gid mode <<< "${out}"
    mode=$(( 8#${mode} ))
    # Select one permission class; denied owner/group access cannot fall through.
    # Shift its bits down, then mask with 3 (binary 011 = w+x).
    if [ "${uid}" = "${WANT_UID}" ]; then
        (( (mode >> 6 & 3) == 3 ))
    elif [ "${gid}" = "${WANT_GID}" ]; then
        (( (mode >> 3 & 3) == 3 ))
    else
        (( (mode & 3) == 3 ))
    fi
}

# Fallback for ACL-granted access: check as the user resolved from WANT_UID.
# Only used after the mode check fails; su is optional.
glance_can_write_acl() {
    local user
    command -v su >/dev/null 2>&1 || return 1
    user="$(getent passwd "${WANT_UID}" | cut -d: -f1)"
    [ -n "${user}" ] || return 1
    su -s /bin/sh "${user}" -c 'test -w "$1" && test -x "$1"' sh "$1" >/dev/null 2>&1
}

fail() {
    echo "ERROR: ${DATADIR} is not writable by ${WANT_UID}:${WANT_GID}." >&2
    echo "  ls -ld: $(ls -ld "${DATADIR}" 2>&1)" >&2
    echo "  The kubelet does not apply fsGroup to NFS volumes, so the export must be" >&2
    echo "  owned by ${WANT_UID}:${WANT_GID}, or group writable and searchable (rwx)" >&2
    echo "  by ${WANT_GID} on the NFS server. Under root_squash this cannot be fixed" >&2
    echo "  from the client." >&2
    exit 1
}

if [ ! -d "${DATADIR}" ]; then
    echo "ERROR: ${DATADIR} is not an existing directory. Check the configured datadir and volume mount." >&2
    exit 1
fi

if glance_can_write "${DATADIR}"; then
    echo "${DATADIR} is already writable by ${WANT_UID}:${WANT_GID} ($(stat -c '%U:%G %A' "${DATADIR}")), nothing to do"
    exit 0
fi

echo "Setting ownership of ${DATADIR} from $(stat -c '%u:%g' "${DATADIR}") to ${WANT_UID}:${WANT_GID}"
# Non-recursive: uploads and deletions need datadir write/search access;
# downloads need readable image files, which this script assumes and does not
# check. Avoid traversing every image over NFS on each init-container run.
# Repair migrated image ownership once during separate maintenance,
# preferably on the NFS server.
if ! chown "${WANT_UID}:${WANT_GID}" "${DATADIR}"; then
    echo "WARNING: chown of ${DATADIR} failed; most likely a root_squash export." >&2
    glance_can_write_acl "${DATADIR}" || fail
    echo "${DATADIR} is writable by ${WANT_UID} via an ACL, continuing"
    exit 0
fi

# Re-check access even after chown succeeds; the server may squash ownership.
if ! glance_can_write "${DATADIR}"; then
    glance_can_write_acl "${DATADIR}" || fail
fi

echo "${DATADIR} is now owned by $(stat -c '%u:%g' "${DATADIR}")"
