# Superphenix Velero Plugin

This repository contains a Velero plugin designed to enhance the backup and restore process for KubeVirt VirtualMachines (VMs) running on clusters using Kube-OVN as the CNI.

## Overview

When Velero backups a KubeVirt VM, it typically captures the VM's specification and its disks. However, in environments using Kube-OVN, the VM's network identity (MAC and IP addresses) is often managed via OVN-specific Custom Resources (`IP` objects). If these are not preserved, a restored VM might receive a different IP or MAC address, which can break services relying on static network identities.

The **Superphenix Velero Plugin** implements a `BackupItemAction` for `virtualmachines.kubevirt.io`. During the backup process, it:
1. Identifies all network interfaces of the VM (Default and NAD/Multus).
2. Retrieves the corresponding Kube-OVN `IP` custom resources.
3. Injects the MAC and IP information into the VM's template annotations.

Upon restoration, Kube-OVN will read these annotations and re-assign the same MAC and IP addresses to the VM's interfaces.

## Features

- **Automatic Network Persistence**: Automatically captures Kube-OVN network settings during backup.
- **Support for Multiple Networks**: Handles both the default OVN network and secondary networks attached via NetworkAttachmentDefinitions (NAD/Multus).
- **KubeVirt Integration**: Works with KubeVirt VMs and integrates with the official `kubevirt-velero-plugin` for consistency checks.

## How it Works

The plugin identifies the network interfaces of a VM by inspecting its `spec.template.spec.networks`. For each interface, it determines the appropriate Kube-OVN `IP` resource name based on the network type:
- **Default Network**: Follows the pattern `{vm-name}.{vm-namespace}`.
- **NAD Network**: Follows the pattern `{vm-name}.{vm-namespace}.{nad-name}.{nad-namespace}.ovn`.

It then fetches these `IP` resources and adds the following annotations to the VM template:
- `[NAD-Annotation]/mac_address`: The MAC address of the interface.
- `[NAD-Annotation]/ip_address`: The IP address(es) of the interface.

## Installation

To use this plugin, you need to add it to your Velero installation.

For example, using the Velero Helm chart:
```yaml
initContainers:
  # Plugin to connect to S3 buckets
  - name: velero-plugin-for-aws
    image: velero/velero-plugin-for-aws:v1.12.1
    imagePullPolicy: IfNotPresent
    volumeMounts:
      - mountPath: /target
        name: plugins
  # Plugin to handle SPX specific backup actions
  - name: velero-plugin-for-superphenix
    image: ghcr.io/super-phenix/superphenix-velero-plugin:latest
    imagePullPolicy: IfNotPresent
    volumeMounts:
      - mountPath: /target
        name: plugins
```

## Local Development

### Prerequisites

- Go 1.25.6 or later
- Access to a Kubernetes cluster with KubeVirt and Kube-OVN (for testing)

> [!NOTE]
> You cannot run the plugin outside of Velero, it needs to be imported as a Velero plugin.

### Testing

Run the unit tests:

```bash
go test ./...
```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
