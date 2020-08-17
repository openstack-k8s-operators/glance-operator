set -e

# This script generates the glance-passwords.conf file and copies the result
# to the ephemeral /var/lib/emptydir volume (mounted by your init container).
# 
# Secrets are obtained from ENV variables.
# ENV variables specify the TransportPassword along with the Database host.
export DatabasePassword=${DatabasePassword:?"Please specify a DatabasePassword variable."}
export TransportPassword=${TransportPassword:?"Please specify a TransportPassword variable."}
export GlanceKeystoneAuthPassword=${GlanceKeystoneAuthPassword:?"Please specify a GlanceKeystoneAuthPassword variable."}
export DatabaseHost=${DatabaseHost:?"Please specify a DatabaseHost variable."}
export TransportHost=${TransportHost:?"Please specify a TransportHost variable."}
export DatabaseUser=${DatabaseUser:-"glance"}
export DatabaseSchema=${DatabaseSchema:-"glance"}

cat > /var/lib/emptydir/glance-passwords.conf <<-EOF_CAT
[DEFAULT]
transport_url=rabbit://guest:$TransportPassword@$TransportHost:5672/?ssl=0

[database]
connection=mysql+pymysql://$DatabaseUser:$DatabasePassword@$DatabaseHost/$DatabaseSchema

[keystone_authtoken]
password=$GlanceKeystoneAuthPassword

[oslo_messaging_notifications]
transport_url=rabbit://guest:$TransportPassword@$TransportHost:5672/?ssl=0
EOF_CAT
