package util

import (
	"context"
	"fmt"
	"os"
	"strings"

	kubeovnv1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	clientset "github.com/kubeovn/kube-ovn/pkg/client/clientset/versioned"
	kubeovnclient "github.com/kubeovn/kube-ovn/pkg/client/clientset/versioned/typed/kubeovn/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultNetworkAnnotation = "ovn.kubernetes.io"
	defaultNetworkPattern    = "%s.%s"
	nadNetworkPattern        = "%s.%s.%s.%s.ovn"
)

type KubeOvnClient interface {
	KubeovnV1() kubeovnclient.KubeovnV1Interface
}

var GetKubeOvnClient = func() (KubeOvnClient, error) {
	kubeConfig := os.Getenv("KUBECONFIG")
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}

	return clientset.NewForConfig(cfg)
}

// NetInfo represents the network information for a VM interface
type NetInfo struct {
	NADAnnotation string
	MAC           string
	IPs           string
}

// GetIPForVM retrieves the IP custom resource associated with a VM's network annotation, name, and namespace.
// We expect the NAD annotation to be the key of an annotation used by Kube-OVN to express settings on an interface.
// For example, mysubnet.mynamespace.ovn.kubernetes.io or ovn.kubernetes.io
func GetIPForVM(nadAnnotation, vmName, vmNamespace string) (*kubeovnv1.IP, error) {
	client, err := GetKubeOvnClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kube-OVN clientset: %w", err)
	}

	// Convert the vmName/vmNamespace and the network annotation of one of its interfaces to the matching IP CustomResource
	ipName, err := getIPCRNameForVM(nadAnnotation, vmName, vmNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve IP name for VM %s/%s: %w", vmNamespace, vmName, err)
	}

	// Retrieve the IP custom resource for that interface/VM
	ip, err := client.KubeovnV1().IPs().Get(context.Background(), ipName, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve the IP custom resource for VM %s/%s: %w", vmNamespace, vmName, err)
	}

	return ip, nil
}

// GetIPsForDefaultNetwork retrieves the IPs for a VM on the default network.
func GetIPsForDefaultNetwork(vmName, vmNamespace string) ([]kubeovnv1.IP, error) {
	ip, err := GetIPForVM(defaultNetworkAnnotation, vmName, vmNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve IP for VM %s/%s: %w", vmNamespace, vmName, err)
	}

	return []kubeovnv1.IP{*ip}, nil
}

// getIPCRNameForVM generates the IP CR name for a VM based on its network annotation, name, and namespace.
// Supports both default networks and networks attached via NetworkAttachmentDefinition (NAD).
func getIPCRNameForVM(nadAnnotation, vmName, vmNamespace string) (string, error) {
	// Kube-OVN annotations must end with "ovn.kubernetes.io", otherwise, we're not dealing with a CNI we can handle here.
	if !strings.HasSuffix(nadAnnotation, defaultNetworkAnnotation) {
		return "", fmt.Errorf("invalid network annotation, expected '%s' to have suffix %s", nadAnnotation, defaultNetworkAnnotation)
	}

	// If the annotation is equal to "ovn.kubernetes.io", we're dealing with a default network.
	// We can discard the annotation as it doesn't contain any useful information to infer
	// the name of the IP custom resource.
	if nadAnnotation == defaultNetworkAnnotation {
		return getIPNameForDefaultNetwork(vmName, vmNamespace)
	}

	// We're dealing with a network attached through a NAD (NetworkAttachmentDefinition).
	// We need to handle it differently from the default networks.
	return getIPNameForNADNetwork(nadAnnotation, vmName, vmNamespace)
}

// getIPNameForDefaultNetwork generates the IP CR name for a VM's default network using its name and namespace.
// Returns the formatted IP name or an error if either VM name or namespace is empty.
func getIPNameForDefaultNetwork(vmName, vmNamespace string) (string, error) {
	if vmName == "" || vmNamespace == "" {
		return "", fmt.Errorf("expected a VM name/namespace, got '%s' and '%s'", vmName, vmNamespace)
	}

	return fmt.Sprintf(defaultNetworkPattern, vmName, vmNamespace), nil
}

// getIPNameForNADNetwork generates the IP CR name for a VM attached through a NetworkAttachmentDefinition.
// Returns the formatted IP name or an error if either VM name or namespace is empty.
func getIPNameForNADNetwork(nadAnnotation, vmName, vmNamespace string) (string, error) {
	if vmName == "" || vmNamespace == "" {
		return "", fmt.Errorf("expected a VM name/namespace, got '%s' and '%s'", vmName, vmNamespace)
	}

	// We remove the useless prefix at the end of the annotation to extract only the information we need
	annotation, found := strings.CutSuffix(nadAnnotation, "."+defaultNetworkAnnotation)
	if !found {
		return "", fmt.Errorf("expected NAD annotation to end with .ovn.kubernetes.io, got %s", annotation)
	}

	// We expect to arrive here with an annotation of pattern [NAD].[NS]
	// We need to extract the name of NAD and namespace.
	split := strings.Split(annotation, ".")
	if len(split) != 2 {
		return "", fmt.Errorf("expected NAD annotation to have pattern [NAD].[NS], got %s", annotation)
	}

	// NAD and VM must be in the same namespace, otherwise something is wrong
	nadName, nadNamespace := split[0], split[1]
	if vmNamespace != nadNamespace {
		return "", fmt.Errorf("expected NAD to be in the same namespace as the VM, got %s for NAD and %s for VM", nadNamespace, vmNamespace)
	}

	return fmt.Sprintf(nadNetworkPattern, vmName, vmNamespace, nadName, nadNamespace), nil
}

// NetworkNameToNadAnnotation translates a Kubevirt NetworkName into a NAD annotation
func NetworkNameToNadAnnotation(networkName string) (string, error) {
	split := strings.Split(networkName, "/")
	if len(split) != 2 {
		return "", fmt.Errorf("expected network name to have format [NS]/[NAD], got %s", networkName)
	}

	return fmt.Sprintf("%s.%s.%s", split[1], split[0], defaultNetworkAnnotation), nil
}

// IPToNetInfo translates a Kube-OVN IP CR into a NetInfo
func IPToNetInfo(nadAnnotation string, ip kubeovnv1.IP) *NetInfo {
	var ips []string
	if ip.Spec.V4IPAddress != "" {
		ips = append(ips, ip.Spec.V4IPAddress)
	}
	if ip.Spec.V6IPAddress != "" {
		ips = append(ips, ip.Spec.V6IPAddress)
	}

	return &NetInfo{
		NADAnnotation: nadAnnotation,
		MAC:           ip.Spec.MacAddress,
		IPs:           strings.Join(ips, ","),
	}
}

// ToAnnotations translates a NetInfo into the corresponding Kube-OVN annotations
func (n *NetInfo) ToAnnotations() map[string]string {
	return map[string]string{
		fmt.Sprintf("%s/%s", n.NADAnnotation, "mac_address"): n.MAC,
		fmt.Sprintf("%s/%s", n.NADAnnotation, "ip_address"):  n.IPs,
	}
}
