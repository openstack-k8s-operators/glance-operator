set -e

# This script generates the glance-passwords.conf file and copies the result
# to the ephemeral /var/lib/emptydir volume (mounted by your init container).
# 
# Secrets are obtained from ENV variables.
export DatabasePassword=${DatabasePassword:?"Please specify a DatabasePassword variable."}
export TransportUrl=${TransportUrl:?"Please specify a TransportUrl variable."}
export GlanceKeystoneAuthPassword=${GlanceKeystoneAuthPassword:?"Please specify a GlanceKeystoneAuthPassword variable."}
export DatabaseHost=${DatabaseHost:?"Please specify a DatabaseHost variable."}
export DatabaseUser=${DatabaseUser:-"glance"}
export DatabaseSchema=${DatabaseSchema:-"glance"}

cat > /var/lib/emptydir/glance-passwords.conf <<-EOF_CAT
[DEFAULT]
transport_url=$TransportUrl

[database]
connection=mysql+pymysql://$DatabaseUser:$DatabasePassword@$DatabaseHost/$DatabaseSchema

[keystone_authtoken]
password=$GlanceKeystoneAuthPassword

[oslo_messaging_notifications]
transport_url=$TransportUrl
EOF_CAT
