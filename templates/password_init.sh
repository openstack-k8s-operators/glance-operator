set -e

# This script generates the glance-passwords.conf file and copies the result
# to the ephemeral /var/lib/emptydir volume (mounted by your init container).
# 
# Secrets are obtained from the /var/lib/secrets/ volume (also mounted by the
# init container.
# ENV variables specify the TransportPassword along with the Database host.

for X in DatabasePassword TransportPassword; do
  if [[ ! -f "/var/lib/secrets/$X" ]] ; then
     echo "Missing secret for $X. Please specify this secret and try again."
     exit 1
  fi
done

export DatabasePassword="$(cat /var/lib/secrets/DatabasePassword)"
export TransportPassword="$(cat /var/lib/secrets/TransportPassword)"

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
password=$DatabasePassword

[oslo_messaging_notifications]
transport_url=rabbit://guest:$TransportPassword@$TransportHost:5672/?ssl=0
EOF_CAT
