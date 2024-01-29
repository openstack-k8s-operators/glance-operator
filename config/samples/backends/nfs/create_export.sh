#!/bin/bash

DEBUG=0
NFS_NET=${NFS_NET:-"172.18.0.0/24"}
NFS_NET_INTERFACE=enp6s0.21
NFS_EXPORT=/var/nfs

function create_export() {
    sudo mkdir -p ${NFS_EXPORT}
    sudo chmod 755 ${NFS_EXPORT}
cat > /tmp/exports <<EOF
${NFS_EXPORT}  ${NFS_NET}(rw,sync,no_root_squash)
EOF

    sudo mv /tmp/exports /etc/exports
    sudo exportfs -a
}

function iptables_apply() {
    sudo iptables -I INPUT -s "${NFS_NET}" -p tcp --dport 32765:32768 -j ACCEPT
    sudo iptables -I INPUT -s "${NFS_NET}" -p udp --dport 32765:32768 -j ACCEPT
    sudo iptables -I INPUT -s "${NFS_NET}" -p tcp --dport 2049 -j ACCEPT
    sudo iptables -I INPUT -s "${NFS_NET}" -p udp --dport 2049 -j ACCEPT
    sudo iptables -I INPUT -s "${NFS_NET}" -p tcp --dport 111 -j ACCEPT
    sudo iptables -I INPUT -s "${NFS_NET}" -p udp --dport 111 -j ACCEPT
}

function enable_nfs_server() {
    # Patch the default config: grab the ip address
    IP_ADDR=$(ip -j -4 a | jq -r --arg iface $NFS_NET_INTERFACE '.[]|select(.ifname == $iface)|.addr_info[0].local')
    sudo sed -i 's/\(\# host\=\)/host= '"$IP_ADDR"'/' /etc/nfs.conf
    sudo systemctl enable nfs-server
    sudo systemctl restart nfs-server
}


echo "Enable nfs-server"
enable_nfs_server

echo "Apply iptables"
iptables_apply

echo "Create the NFS export"
create_export
if [ "$DEBUG" -eq 1 ]; then
    showmount -e
    ss -antop | grep 2049
fi
