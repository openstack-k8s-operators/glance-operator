#!/bin/bash
set -ex

oc delete validatingwebhookconfiguration/vglance.kb.io --ignore-not-found
oc delete mutatingwebhookconfiguration/mglance.kb.io --ignore-not-found
oc delete validatingwebhookconfiguration/vglanceapi.kb.io --ignore-not-found
oc delete mutatingwebhookconfiguration/mglanceapi.kb.io --ignore-not-found
