package util

import (
	"fmt"

	kubeovnv1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	v1 "kubevirt.io/api/core/v1"
)

// GetKubeovnAnnotationsForVM returns the Kube-OVN annotations to set on the VM to persist its MAC and IP addresses
func GetKubeovnAnnotationsForVM(vm *v1.VirtualMachine) (map[string]string, error) {
	if vm == nil {
		return nil, fmt.Errorf("VM object is nil")
	}
	netInfo, err := GetNetInfoForVm(vm)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve netInfo for VM %s/%s: %w", vm.Namespace, vm.Name, err)
	}

	annotations := make(map[string]string)

	for _, netInf := range netInfo {
		anns := netInf.ToAnnotations()
		for k, v := range anns {
			annotations[k] = v
		}
	}

	return annotations, nil
}

// GetNetInfoForVm returns the IPs and NAD annotations of the VM's interfaces
func GetNetInfoForVm(vm *v1.VirtualMachine) ([]NetInfo, error) {
	ips, nads, err := GetIPsForVM(vm)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve IP CRs for VM %s/%s", vm.Namespace, vm.Name)
	}

	var netInfos []NetInfo
	for i, ip := range ips {
		netInfos = append(netInfos, *IPToNetInfo(nads[i], ip))
	}

	return netInfos, nil
}

// GetIPsForVM returns the IPs of the VM's interfaces and the corresponding NAD for each
func GetIPsForVM(vm *v1.VirtualMachine) ([]kubeovnv1.IP, []string, error) {
	// No network on the VM means it will inherit the default network and only the default network
	if len(vm.Spec.Template.Spec.Networks) == 0 {
		ips, err := GetIPsForDefaultNetwork(vm.Name, vm.Namespace)
		return ips, []string{defaultNetworkAnnotation}, err
	}

	multusIsPrimary := false
	explicitPodNetwork := false
	var ips []kubeovnv1.IP
	var nads []string

	// Pass over every network defined in the specs and extract its IP CustomResource
	for _, network := range vm.Spec.Template.Spec.Networks {
		// We're mounting the default network of the cluster on one of the interfaces
		if network.Pod != nil {
			explicitPodNetwork = true
			ip, err := GetIPsForDefaultNetwork(vm.Name, vm.Namespace)
			if err != nil {
				return nil, nil, err
			}

			ips = append(ips, ip...)
			nads = append(nads, defaultNetworkAnnotation)
		}

		// We're dealing with Multus interfaces, they may be secondary or primary
		if network.Multus != nil {
			if network.Multus.Default {
				multusIsPrimary = true
			}

			// Convert the Kubevirt NetworkName to a NAD annotation
			nadAnnotation, err := NetworkNameToNadAnnotation(network.Multus.NetworkName)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid network name for vm %s/%s: %w", vm.Namespace, vm.Name, err)
			}

			ip, err := GetIPForVM(nadAnnotation, vm.Name, vm.Namespace)
			if err != nil {
				return nil, nil, err
			}

			ips = append(ips, *ip)
			nads = append(nads, nadAnnotation)
		}
	}

	// If no Multus interface is primary, a default interface will be injected
	if !multusIsPrimary && !explicitPodNetwork {
		ip, err := GetIPsForDefaultNetwork(vm.Name, vm.Namespace)
		if err != nil {
			return nil, nil, err
		}

		ips = append(ips, ip...)
		nads = append(nads, defaultNetworkAnnotation)
	}

	return ips, nads, nil
}
