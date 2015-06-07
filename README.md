### CfgGen
#### generate configuration


##### Notice: This configuration may not work at all without testing.
##### Do not use this without testing in a safe environment

    Usage: 

    URL http://host:port/{ipaddress} will generate a configuration
    text with a configuration and inject into a backend registry
    defaulting to etcd.

    Loads template from a text 'yaml' file with golang formatted
    templates.

    The initial use case is a coreos cloud init formatted file.

    http get:
    http://host:port

        returns this document

    http get:
    http://host:port/10.0.0.3

        returns the --config-template after replacing golang template
        format options with a configuration then injects into a
        backend registry defaulting to etcd with the path
        /config/by_ip/{ipaddress} equivalent to

        etcdctl set /config/by_ip/{ipaddress} "{configuration json}"


    http get:
    http://host:port/{ipaddress}

        loads --config-file json injecting to Cfg struct executes the
        template replacing golang formatted template entries from the
        --config-template file, writes the modified json config to
        etcd in /config/by_ip/{ipaddress} and responds with the result
        of the template cloud config or other template formatted file
        after replacement.

    http get:
    http://host:port/config/

        dumps the raw json from the --config-file

    http get:
    http://host:port/config/{ipaddress}

        dumps the --config-file's json after formatting and modifying
        it with the ipaddress and subnetbits

    http get:
    http://host:port/etcd/{ipaddress} 

        dumps the etcd record after modification for {ipaddress} if
        found as if using etcdctl to pull down data, formatting with
        indentation.

        etcdctl get /config/by_ip/{ipaddress} | jq .
    
    
    http get:
    http://host:port/etcd/

        dumps all etcd records after modification for {ipaddress} if
        found as if using etcdctl to pull down data, formatting with
        indentation.

        etcdctl get --recursive /config/by_ip/{ipaddress} | jq .
    
    
    http get:
    http://host:port/plain/{filename} 

        read a raw file


    Example commands to query configuration state
    curl -s localhost:8080/config/1.1.1.1

    etcdctl ls --recursive /config/by_ip/

    for p in $(etcdctl ls --recursive /config/by_ip); do etcdctl get ${p}; done

### CfgGen
#### example initial json definition

    Replacement options are managed via config.json in config format

    type v0Cfg struct {
        CfgVersion            string             `json:CfgVersion:`
        HostIp                string             `json:HostIp:`
        HostName              string             `json:HostName:`
        HostSubnetBits        string             `json:HostSubnetBits:`
        HostCIDR              string             `json:HostCIDR:`

        Gateway               string             `json:Gateway:`

        DnsSearch             string             `json:DnsSearch:`
        Domain                string             `json:Domain:`
        Dns1                  string             `json:Dns1:`
        Dns2                  string             `json:Dns2:`

        // restricted access
        ClusterMgmtIp         string             `json:ClusterMgmtIp:`
        ClusterMgmtPort       string             `json:ClusterMgmtPort:`
        ClusterMgmtUrl        string             `json:ClusterMgmtUrl:`

        DockerIp              string             `json:DockerIp:`
        DockerSubnetBits      string             `json:DockerSubnetBits:`
        DockerCIDR            string             `json:DockerCIDR:`
        DockerMgmtPort        string             `json:DockerMgmtPort:`
        DockerMgmtUrl         string             `json:DockerMgmtUrl:`

        EtcdIp                string             `json:EtcdIp:`
        Etcdv1ClientPort      string             `json:Etcdv1ClientPort:`
        Etcdv1MgmtPort        string             `json:Etcdv1MgmtPort:`
        Etcdv2ClientPort      string             `json:Etcdv2ClientPort:`
        Etcdv2MgmtPort        string             `json:Etcdv2MgmtPort:`
        EtcdPeers             string             `json:EtcdPeers:`
        EtcdDiscoveryToken    string             `json:EtcdDiscoveryToken:`
        EtcdDiscoveryUrl      string             `json:EtcdDiscoveryUrl:`

        FleetMgmtUrl          string             `json:FleetMgmtUrl:`

        KubeletOptions        string             `json:KubeletOptions:`
        PrivilegedOption      string             `json:PrivilegedOption:`

        ProxyUrl              string             `json:ProxyUrl:`       

        Metadata              MetadataMap        `json:Metadata:`
    }

### CfgGen
#### template example

    Example template snippet 

    #cloud-config

    # Notice: This configuration may not work at all without testing.
    # Do not use this without testing in a safe environment

    # This configuration uses a bridge interface bridge0 to enable
    # configuring docker ip address assignment visibility directly on the
    # network, a bonded interface for ethernet device failover maps to the
    # underlying ether net devices and CIDR --fixed-cidr to partition a
    # subnet at defined number of docker subnet bits.

    # There are also some configuration options to enable managing via an
    # early version of kubernetes and to configure options. These options
    # are changing, so YMMV.

    write_files:
      - path: /etc/resolv.conf
        permissions: 0644
        owner: root
        content: |
          search {{ .DnsSearch }}
          domain {{ .Domain }}
          nameserver {{ .Dns1 }}
          nameserver {{ .Dns2 }}

      - path: /var/lib/ecmi/bin/iptables-managment.sh
        permissions: 0644
        owner: root
        content: |
          #!/bin/bash
          iptables -I INPUT -p tcp -d {{ .HostIp }} --dport {{ .DockerMgmtPort }} -j DROP 
          iptables -I INPUT -i eth0 -p tcp -s {{ .ClusterMgmtIp }} -d {{ .HostIp }} \
            --dport {{ .DockerMgmtPort }} -j ACCEPT 

    coreos:

      update:
        reboot-strategy: off
        group: stable

      fleet:
        etcd_servers: {{ .FleetMgmtUrl }}
        metadata: "{{ .Metadata | Commaize }}"

      etcd:
        name: etcd
        bind-addr: 0.0.0.0
        peer-addr: {{ .EtcdIp }}:7001
        addr: {{ .EtcdIp }}:4001
        cluster-active-size: 1
        snapshot: true
        data-dir: /var/lib/etcd

      units:

        - name: systemd-journald.service
          command: restart

        - name: etcd.service
          command: start

        - name: fleet.service
          command: start
          drop-ins:
            - name: 50-fleet-install.conf
              content: |
               [Install]
               WantedBy=multi-user.target

        - name: 10.static.netdev
          command: start
          content: |
            [NetDev]
            Name=bridge0
            Kind=bridge

        - name: 20.static.network
          command: start
          content: |
            [Match]
            Name=bridge0

            [Network]
            Address={{ .HostIp }}/{{ .HostSubnetBits }}
            DNS={{ .Dns1 }}
            DNS={{ .Dns2 }}
            Gateway={{ .Gateway }}



    ### Sample Json written to Etcd


    for p in $(etcdctl ls --recursive /config/by_ip); do etcdctl get ${p}; done

    The json provided describes the current desired configuration
    state of the host with etcd as the system of record.

    {
      "CfgVersion": "v0",
      "HostIp": "10.1.2.65",
      "HostName": "coreos-001",
      "HostSubnetBits": "8",
      "HostCIDR": "10.1.2.65/23",
      "Gateway": "10.1.3.1",
      "DnsSearch": "example.com canary.example.com",
      "Domain": "example.com",
      "Dns1": "10.1.4.3",
      "Dns2": "10.1.4.4",
      "ClusterMgmtIp": "10.1.3.19",
      "ClusterMgmtPort": "8080",
      "ClusterMgmtUrl": "http://10.1.3.19:8080",
      "DockerIp": "10.1.2.193",
      "DockerSubnetBits": "28",
      "DockerCIDR": "10.1.2.193/28",
      "DockerMgmtPort": "2375",
      "DockerMgmtUrl": "tcp://10.1.2.65:2375",
      "EtcdIp": "10.1.3.19",
      "Etcdv1ClientPort": "4001",
      "Etcdv1MgmtPort": "7001",
      "Etcdv2ClientPort": "2379",
      "Etcdv2MgmtPort": "2380",
      "EtcdPeers": "http://10.1.3.19:4001",
      "EtcdDiscoveryToken": "4421819c7e708956af7ab67ddcd7ace3",
      "EtcdDiscoveryUrl": "http://10.1.3.19/4421819c7e708956af7ab67ddcd7ace3",
      "FleetMgmtUrl": "http://10.1.3.19:4001",
      "KubeletOptions": "--etcd_servers=http://10.1.3.19:8080",
      "PrivilegedOption": "",
      "ProxyUrl": "http://userid:proxypass@example.com:80",
      "Metadata": {
        "arch": "x86_64",
        "country": "US",
        "datacenter": "sftlyr:dal01",
        "engines": "docker",
        "hostname": "coreos-001",
        "mgmt": "wkr",
        "mntr": "false",
        "platform": "HP G8 12 Core",
        "public_ip": "10.1.2.65",
        "region": "central",
        "role": "node",
        "zone": "private"
      }
    }

### CfgGen
#### LICENSE

[License](plain/LICENSE)
