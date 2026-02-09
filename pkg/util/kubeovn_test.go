package util

import (
	"context"
	"fmt"
	"testing"

	kubeovnv1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/kubeovn/kube-ovn/pkg/client/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetIPNameForDefaultNetwork(t *testing.T) {
	tests := []struct {
		name        string
		vmName      string
		vmNamespace string
		want        string
		wantErr     bool
	}{
		{
			name:        "valid vm name and namespace",
			vmName:      "test-vm",
			vmNamespace: "test-ns",
			want:        "test-vm.test-ns",
			wantErr:     false,
		},
		{
			name:        "empty vm name",
			vmName:      "",
			vmNamespace: "test-ns",
			want:        "",
			wantErr:     true,
		},
		{
			name:        "empty vm namespace",
			vmName:      "test-vm",
			vmNamespace: "",
			want:        "",
			wantErr:     true,
		},
		{
			name:        "both empty",
			vmName:      "",
			vmNamespace: "",
			want:        "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getIPNameForDefaultNetwork(tt.vmName, tt.vmNamespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("getIPNameForDefaultNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getIPNameForDefaultNetwork() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetIPNameForNADNetwork(t *testing.T) {
	tests := []struct {
		name          string
		nadAnnotation string
		vmName        string
		vmNamespace   string
		want          string
		wantErr       bool
	}{
		{
			name:          "valid nad annotation, vm name and namespace",
			nadAnnotation: "test-nad.test-ns.ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			want:          "test-vm.test-ns.test-nad.test-ns",
			wantErr:       false,
		},
		{
			name:          "invalid nad annotation suffix",
			nadAnnotation: "test-nad.test-ns.invalid",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			want:          "",
			wantErr:       true,
		},
		{
			name:          "invalid nad annotation pattern (missing namespace)",
			nadAnnotation: "test-nad.ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			want:          "",
			wantErr:       true,
		},
		{
			name:          "nad namespace mismatch with vm namespace",
			nadAnnotation: "test-nad.other-ns.ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			want:          "",
			wantErr:       true,
		},
		{
			name:          "empty vm name",
			nadAnnotation: "test-nad.test-ns.ovn.kubernetes.io",
			vmName:        "",
			vmNamespace:   "test-ns",
			want:          "",
			wantErr:       true,
		},
		{
			name:          "empty vm namespace",
			nadAnnotation: "test-nad.test-ns.ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "",
			want:          "",
			wantErr:       true,
		},
		{
			name:          "annotation too long (more than 2 parts before suffix)",
			nadAnnotation: "part1.part2.part3.ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			want:          "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getIPNameForNADNetwork(tt.nadAnnotation, tt.vmName, tt.vmNamespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("getIPNameForNADNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("getIPNameForNADNetwork() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetIPCRNameForVM(t *testing.T) {
	tests := []struct {
		name          string
		nadAnnotation string
		vmName        string
		vmNamespace   string
		want          string
		wantErr       bool
	}{
		{
			name:          "default network annotation",
			nadAnnotation: "ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			want:          "test-vm.test-ns",
			wantErr:       false,
		},
		{
			name:          "NAD network annotation",
			nadAnnotation: "test-nad.test-ns.ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			want:          "test-vm.test-ns.test-nad.test-ns",
			wantErr:       false,
		},
		{
			name:          "invalid suffix",
			nadAnnotation: "invalid.suffix",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			want:          "",
			wantErr:       true,
		},
		{
			name:          "error from getIPNameForDefaultNetwork (empty name)",
			nadAnnotation: "ovn.kubernetes.io",
			vmName:        "",
			vmNamespace:   "test-ns",
			want:          "",
			wantErr:       true,
		},
		{
			name:          "error from getIPNameForNADNetwork (namespace mismatch)",
			nadAnnotation: "test-nad.other-ns.ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			want:          "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getIPCRNameForVM(tt.nadAnnotation, tt.vmName, tt.vmNamespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("getIPCRNameForVM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("getIPCRNameForVM() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetIPForVM(t *testing.T) {
	// Mock GetKubeOvnClient
	originalGetKubeOvnClient := GetKubeOvnClient
	defer func() { GetKubeOvnClient = originalGetKubeOvnClient }()

	tests := []struct {
		name          string
		nadAnnotation string
		vmName        string
		vmNamespace   string
		existingIPs   []*kubeovnv1.IP
		clientErr     error
		wantIPName    string
		wantErr       bool
	}{
		{
			name:          "successfully retrieve IP for default network",
			nadAnnotation: "ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
				},
			},
			wantIPName: "test-vm.test-ns",
			wantErr:    false,
		},
		{
			name:          "successfully retrieve IP for NAD network",
			nadAnnotation: "test-nad.test-ns.ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.test-nad.test-ns",
					},
				},
			},
			wantIPName: "test-vm.test-ns.test-nad.test-ns",
			wantErr:    false,
		},
		{
			name:          "failed to create Kube-OVN client",
			nadAnnotation: "ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			clientErr:     fmt.Errorf("client creation failed"),
			wantErr:       true,
		},
		{
			name:          "invalid network annotation",
			nadAnnotation: "invalid-annotation",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			wantErr:       true,
		},
		{
			name:          "IP resource not found",
			nadAnnotation: "ovn.kubernetes.io",
			vmName:        "test-vm",
			vmNamespace:   "test-ns",
			existingIPs:   []*kubeovnv1.IP{},
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up fake client
			fakeClient := fake.NewSimpleClientset()
			for _, ip := range tt.existingIPs {
				_, _ = fakeClient.KubeovnV1().IPs().Create(context.Background(), ip, metav1.CreateOptions{})
			}

			GetKubeOvnClient = func() (KubeOvnClient, error) {
				if tt.clientErr != nil {
					return nil, tt.clientErr
				}
				return fakeClient, nil
			}

			got, err := GetIPForVM(tt.nadAnnotation, tt.vmName, tt.vmNamespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIPForVM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == nil {
					t.Errorf("GetIPForVM() expected IP, got nil")
				} else if got.Name != tt.wantIPName {
					t.Errorf("GetIPForVM() got IP name = %v, want %v", got.Name, tt.wantIPName)
				}
			}
		})
	}
}

func TestGetIPsForDefaultNetwork(t *testing.T) {
	// Mock GetKubeOvnClient
	originalGetKubeOvnClient := GetKubeOvnClient
	defer func() { GetKubeOvnClient = originalGetKubeOvnClient }()

	tests := []struct {
		name        string
		vmName      string
		vmNamespace string
		existingIPs []*kubeovnv1.IP
		wantErr     bool
	}{
		{
			name:        "successfully retrieve IPs for default network",
			vmName:      "test-vm",
			vmNamespace: "test-ns",
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
				},
			},
			wantErr: false,
		},
		{
			name:        "failed to retrieve IP for default network",
			vmName:      "test-vm",
			vmNamespace: "test-ns",
			existingIPs: []*kubeovnv1.IP{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up fake client
			fakeClient := fake.NewSimpleClientset()
			for _, ip := range tt.existingIPs {
				_, _ = fakeClient.KubeovnV1().IPs().Create(context.Background(), ip, metav1.CreateOptions{})
			}

			GetKubeOvnClient = func() (KubeOvnClient, error) {
				return fakeClient, nil
			}

			got, err := GetIPsForDefaultNetwork(tt.vmName, tt.vmNamespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIPsForDefaultNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != 1 {
					t.Errorf("GetIPsForDefaultNetwork() expected 1 IP, got %d", len(got))
				} else if got[0].Name != fmt.Sprintf("%s.%s", tt.vmName, tt.vmNamespace) {
					t.Errorf("GetIPsForDefaultNetwork() got IP name = %v, want %v", got[0].Name, fmt.Sprintf("%s.%s", tt.vmName, tt.vmNamespace))
				}
			}
		})
	}
}

func TestNetworkNameToNadAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		networkName string
		want        string
		wantErr     bool
	}{
		{
			name:        "valid network name",
			networkName: "test-ns/test-nad",
			want:        "test-nad.test-ns.ovn.kubernetes.io",
			wantErr:     false,
		},
		{
			name:        "invalid network name (missing slash)",
			networkName: "test-ns-test-nad",
			want:        "",
			wantErr:     true,
		},
		{
			name:        "invalid network name (too many slashes)",
			networkName: "test-ns/test-nad/extra",
			want:        "",
			wantErr:     true,
		},
		{
			name:        "empty network name",
			networkName: "",
			want:        "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NetworkNameToNadAnnotation(tt.networkName)
			if (err != nil) != tt.wantErr {
				t.Errorf("NetworkNameToNadAnnotation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NetworkNameToNadAnnotation() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIPToNetInfo(t *testing.T) {
	tests := []struct {
		name          string
		nadAnnotation string
		ip            kubeovnv1.IP
		want          *NetInfo
	}{
		{
			name:          "dual stack IP",
			nadAnnotation: "ovn.kubernetes.io",
			ip: kubeovnv1.IP{
				Spec: kubeovnv1.IPSpec{
					MacAddress:  "00:00:00:00:00:01",
					V4IPAddress: "10.0.0.1",
					V6IPAddress: "fd00::1",
				},
			},
			want: &NetInfo{
				NADAnnotation: "ovn.kubernetes.io",
				MAC:           "00:00:00:00:00:01",
				IPs:           "10.0.0.1,fd00::1",
			},
		},
		{
			name:          "ipv4 only",
			nadAnnotation: "test-nad.test-ns.ovn.kubernetes.io",
			ip: kubeovnv1.IP{
				Spec: kubeovnv1.IPSpec{
					MacAddress:  "00:00:00:00:00:02",
					V4IPAddress: "10.0.0.2",
					V6IPAddress: "",
				},
			},
			want: &NetInfo{
				NADAnnotation: "test-nad.test-ns.ovn.kubernetes.io",
				MAC:           "00:00:00:00:00:02",
				IPs:           "10.0.0.2",
			},
		},
		{
			name:          "ipv6 only",
			nadAnnotation: "ovn.kubernetes.io",
			ip: kubeovnv1.IP{
				Spec: kubeovnv1.IPSpec{
					MacAddress:  "00:00:00:00:00:03",
					V4IPAddress: "",
					V6IPAddress: "fd00::3",
				},
			},
			want: &NetInfo{
				NADAnnotation: "ovn.kubernetes.io",
				MAC:           "00:00:00:00:00:03",
				IPs:           "fd00::3",
			},
		},
		{
			name:          "empty IPs",
			nadAnnotation: "ovn.kubernetes.io",
			ip: kubeovnv1.IP{
				Spec: kubeovnv1.IPSpec{
					MacAddress:  "00:00:00:00:00:04",
					V4IPAddress: "",
					V6IPAddress: "",
				},
			},
			want: &NetInfo{
				NADAnnotation: "ovn.kubernetes.io",
				MAC:           "00:00:00:00:00:04",
				IPs:           "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IPToNetInfo(tt.nadAnnotation, tt.ip)
			if got.NADAnnotation != tt.want.NADAnnotation {
				t.Errorf("IPToNetInfo() NADAnnotation = %v, want %v", got.NADAnnotation, tt.want.NADAnnotation)
			}
			if got.MAC != tt.want.MAC {
				t.Errorf("IPToNetInfo() MAC = %v, want %v", got.MAC, tt.want.MAC)
			}
			if got.IPs != tt.want.IPs {
				t.Errorf("IPToNetInfo() IPs = %v, want %v", got.IPs, tt.want.IPs)
			}
		})
	}
}

func TestNetInfoToAnnotations(t *testing.T) {
	tests := []struct {
		name    string
		netInfo NetInfo
		want    map[string]string
	}{
		{
			name: "default network",
			netInfo: NetInfo{
				NADAnnotation: "ovn.kubernetes.io",
				MAC:           "00:00:00:00:00:01",
				IPs:           "10.0.0.1,fd00::1",
			},
			want: map[string]string{
				"ovn.kubernetes.io/mac_address": "00:00:00:00:00:01",
				"ovn.kubernetes.io/ip_address":  "10.0.0.1,fd00::1",
			},
		},
		{
			name: "NAD network",
			netInfo: NetInfo{
				NADAnnotation: "test-nad.test-ns.ovn.kubernetes.io",
				MAC:           "00:00:00:00:00:02",
				IPs:           "10.0.0.2,",
			},
			want: map[string]string{
				"test-nad.test-ns.ovn.kubernetes.io/mac_address": "00:00:00:00:00:02",
				"test-nad.test-ns.ovn.kubernetes.io/ip_address":  "10.0.0.2,",
			},
		},
		{
			name: "empty values",
			netInfo: NetInfo{
				NADAnnotation: "ovn.kubernetes.io",
				MAC:           "",
				IPs:           ",",
			},
			want: map[string]string{
				"ovn.kubernetes.io/mac_address": "",
				"ovn.kubernetes.io/ip_address":  ",",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.netInfo.ToAnnotations()
			if len(got) != len(tt.want) {
				t.Errorf("NetInfoToAnnotations() got map of length %d, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("NetInfoToAnnotations() key %s: got %s, want %s", k, got[k], v)
				}
			}
		})
	}
}
