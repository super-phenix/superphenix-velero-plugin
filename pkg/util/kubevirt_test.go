package util

import (
	"context"
	"testing"

	kubeovnv1 "github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/kubeovn/kube-ovn/pkg/client/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"
)

func TestGetIPsForVM(t *testing.T) {
	// Mock GetKubeOvnClient
	originalGetKubeOvnClient := GetKubeOvnClient
	defer func() { GetKubeOvnClient = originalGetKubeOvnClient }()

	tests := []struct {
		name        string
		machine     v1.VirtualMachine
		existingIPs []*kubeovnv1.IP
		wantIPNames []string
		wantNads    []string
		wantErr     bool
	}{
		{
			name: "VM with no networks (default network expected)",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
				},
			},
			wantIPNames: []string{"test-vm.test-ns"},
			wantNads:    []string{defaultNetworkAnnotation},
			wantErr:     false,
		},
		{
			name: "VM with explicit Pod network",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{
								{
									Name: "default",
									NetworkSource: v1.NetworkSource{
										Pod: &v1.PodNetwork{},
									},
								},
							},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
				},
			},
			wantIPNames: []string{"test-vm.test-ns"},
			wantNads:    []string{defaultNetworkAnnotation},
			wantErr:     false,
		},
		{
			name: "VM with Multus network (secondary)",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{
								{
									Name: "secondary",
									NetworkSource: v1.NetworkSource{
										Multus: &v1.MultusNetwork{
											NetworkName: "test-ns/test-nad",
										},
									},
								},
							},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.test-nad.test-ns.ovn",
					},
				},
			},
			wantIPNames: []string{"test-vm.test-ns.test-nad.test-ns.ovn", "test-vm.test-ns"},
			wantNads:    []string{"test-nad.test-ns.ovn.kubernetes.io", defaultNetworkAnnotation},
			wantErr:     false,
		},
		{
			name: "VM with Multus network as default (primary)",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{
								{
									Name: "primary",
									NetworkSource: v1.NetworkSource{
										Multus: &v1.MultusNetwork{
											NetworkName: "test-ns/test-nad",
											Default:     true,
										},
									},
								},
							},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.test-nad.test-ns.ovn",
					},
				},
			},
			wantIPNames: []string{"test-vm.test-ns.test-nad.test-ns.ovn"},
			wantNads:    []string{"test-nad.test-ns.ovn.kubernetes.io"},
			wantErr:     false,
		},
		{
			name: "VM with multiple secondary networks",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{
								{
									Name: "secondary1",
									NetworkSource: v1.NetworkSource{
										Multus: &v1.MultusNetwork{
											NetworkName: "test-ns/nad1",
										},
									},
								},
								{
									Name: "secondary2",
									NetworkSource: v1.NetworkSource{
										Multus: &v1.MultusNetwork{
											NetworkName: "test-ns/nad2",
										},
									},
								},
							},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.nad1.test-ns.ovn",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.nad2.test-ns.ovn",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
				},
			},
			wantIPNames: []string{"test-vm.test-ns.nad1.test-ns.ovn", "test-vm.test-ns.nad2.test-ns.ovn", "test-vm.test-ns"},
			wantNads: []string{
				"nad1.test-ns.ovn.kubernetes.io",
				"nad2.test-ns.ovn.kubernetes.io",
				defaultNetworkAnnotation,
			},
			wantErr: false,
		},
		{
			name: "VM with a default multus network",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{
								{
									Name: "primary",
									NetworkSource: v1.NetworkSource{
										Multus: &v1.MultusNetwork{
											NetworkName: "test-ns/nad-primary",
											Default:     true,
										},
									},
								},
							},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.nad-primary.test-ns.ovn",
					},
				},
			},
			wantIPNames: []string{"test-vm.test-ns.nad-primary.test-ns.ovn"},
			wantNads:    []string{"nad-primary.test-ns.ovn.kubernetes.io"},
			wantErr:     false,
		},
		{
			name: "VM with non-explicit default network + a multus secondary",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{
								{
									Name: "secondary",
									NetworkSource: v1.NetworkSource{
										Multus: &v1.MultusNetwork{
											NetworkName: "test-ns/nad-secondary",
										},
									},
								},
							},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.nad-secondary.test-ns.ovn",
					},
				},
			},
			wantIPNames: []string{"test-vm.test-ns.nad-secondary.test-ns.ovn", "test-vm.test-ns"},
			wantNads:    []string{"nad-secondary.test-ns.ovn.kubernetes.io", defaultNetworkAnnotation},
			wantErr:     false,
		},
		{
			name: "Error handling for invalid Multus network name",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{
								{
									Name: "invalid",
									NetworkSource: v1.NetworkSource{
										Multus: &v1.MultusNetwork{
											NetworkName: "invalid-name",
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Error handling for IP retrieval failure",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{}, // No IP in client
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()
			for _, ip := range tt.existingIPs {
				_, _ = fakeClient.KubeovnV1().IPs().Create(context.Background(), ip, metav1.CreateOptions{})
			}

			GetKubeOvnClient = func() (KubeOvnClient, error) {
				return fakeClient, nil
			}

			got, nads, err := GetIPsForVM(&tt.machine)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIPsForVM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != len(tt.wantIPNames) {
					t.Errorf("GetIPsForVM() got %d IPs, want %d", len(got), len(tt.wantIPNames))
					return
				}
				if len(nads) != len(tt.wantNads) {
					t.Errorf("GetIPsForVM() got %d NADs, want %d", len(nads), len(tt.wantNads))
					return
				}
				for i, name := range tt.wantIPNames {
					if got[i].Name != name {
						t.Errorf("GetIPsForVM() got IP[%d].Name = %v, want %v", i, got[i].Name, name)
					}
					if nads[i] != tt.wantNads[i] {
						t.Errorf("GetIPsForVM() got nads[%d] = %v, want %v", i, nads[i], tt.wantNads[i])
					}
				}
			}
		})
	}
}

func TestGetNetInfoForVm(t *testing.T) {
	// Mock GetKubeOvnClient
	originalGetKubeOvnClient := GetKubeOvnClient
	defer func() { GetKubeOvnClient = originalGetKubeOvnClient }()

	tests := []struct {
		name         string
		machine      v1.VirtualMachine
		existingIPs  []*kubeovnv1.IP
		wantNetInfos []NetInfo
		wantErr      bool
	}{
		{
			name: "VM with default network only",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
					Spec: kubeovnv1.IPSpec{
						V4IPAddress: "10.0.0.1",
						MacAddress:  "00:00:00:00:00:01",
					},
				},
			},
			wantNetInfos: []NetInfo{
				{
					NADAnnotation: defaultNetworkAnnotation,
					IPs:           "10.0.0.1",
					MAC:           "00:00:00:00:00:01",
				},
			},
			wantErr: false,
		},
		{
			name: "VM with secondary network",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{
								{
									Name: "secondary",
									NetworkSource: v1.NetworkSource{
										Multus: &v1.MultusNetwork{
											NetworkName: "test-ns/test-nad",
										},
									},
								},
							},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.test-nad.test-ns.ovn",
					},
					Spec: kubeovnv1.IPSpec{
						V4IPAddress: "192.168.1.1",
						MacAddress:  "00:00:00:00:00:02",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
					Spec: kubeovnv1.IPSpec{
						V4IPAddress: "10.0.0.1",
						MacAddress:  "00:00:00:00:00:01",
					},
				},
			},
			wantNetInfos: []NetInfo{
				{
					NADAnnotation: "test-nad.test-ns.ovn.kubernetes.io",
					IPs:           "192.168.1.1",
					MAC:           "00:00:00:00:00:02",
				},
				{
					NADAnnotation: defaultNetworkAnnotation,
					IPs:           "10.0.0.1",
					MAC:           "00:00:00:00:00:01",
				},
			},
			wantErr: false,
		},
		{
			name: "Error handling when IP retrieval fails",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{}, // Missing IP
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()
			for _, ip := range tt.existingIPs {
				_, _ = fakeClient.KubeovnV1().IPs().Create(context.Background(), ip, metav1.CreateOptions{})
			}

			GetKubeOvnClient = func() (KubeOvnClient, error) {
				return fakeClient, nil
			}

			got, err := GetNetInfoForVm(&tt.machine)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNetInfoForVm() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != len(tt.wantNetInfos) {
					t.Errorf("GetNetInfoForVm() got %d NetInfos, want %d", len(got), len(tt.wantNetInfos))
					return
				}
				for i, want := range tt.wantNetInfos {
					if got[i].NADAnnotation != want.NADAnnotation {
						t.Errorf("GetNetInfoForVm() got[%d].NADAnnotation = %v, want %v", i, got[i].NADAnnotation, want.NADAnnotation)
					}
					if got[i].IPs != want.IPs {
						t.Errorf("GetNetInfoForVm() got[%d].IPs = %v, want %v", i, got[i].IPs, want.IPs)
					}
					if got[i].MAC != want.MAC {
						t.Errorf("GetNetInfoForVm() got[%d].MAC = %v, want %v", i, got[i].MAC, want.MAC)
					}
				}
			}
		})
	}
}

func TestGetKubeovnAnnotationsForVM(t *testing.T) {
	// Mock GetKubeOvnClient
	originalGetKubeOvnClient := GetKubeOvnClient
	defer func() { GetKubeOvnClient = originalGetKubeOvnClient }()

	tests := []struct {
		name        string
		machine     v1.VirtualMachine
		existingIPs []*kubeovnv1.IP
		wantAnns    map[string]string
		useNil      bool
		wantErr     bool
	}{
		{
			name: "VM with default network only",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
					Spec: kubeovnv1.IPSpec{
						V4IPAddress: "10.0.0.1",
						MacAddress:  "00:00:00:00:00:01",
					},
				},
			},
			wantAnns: map[string]string{
				"ovn.kubernetes.io/ip_address":  "10.0.0.1",
				"ovn.kubernetes.io/mac_address": "00:00:00:00:00:01",
			},
			wantErr: false,
		},
		{
			name: "VM with multiple networks",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{
								{
									Name: "secondary",
									NetworkSource: v1.NetworkSource{
										Multus: &v1.MultusNetwork{
											NetworkName: "test-ns/test-nad",
										},
									},
								},
							},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
					Spec: kubeovnv1.IPSpec{
						V4IPAddress: "10.0.0.1",
						MacAddress:  "00:00:00:00:00:01",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.test-nad.test-ns.ovn",
					},
					Spec: kubeovnv1.IPSpec{
						V4IPAddress: "10.0.0.2",
						MacAddress:  "00:00:00:00:00:02",
					},
				},
			},
			wantAnns: map[string]string{
				"ovn.kubernetes.io/ip_address":                   "10.0.0.1",
				"ovn.kubernetes.io/mac_address":                  "00:00:00:00:00:01",
				"test-nad.test-ns.ovn.kubernetes.io/ip_address":  "10.0.0.2",
				"test-nad.test-ns.ovn.kubernetes.io/mac_address": "00:00:00:00:00:02",
			},
			wantErr: false,
		},
		{
			name: "Error propagation from GetNetInfoForVm",
			machine: v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: v1.VirtualMachineSpec{
					Template: &v1.VirtualMachineInstanceTemplateSpec{
						Spec: v1.VirtualMachineInstanceSpec{
							Networks: []v1.Network{},
						},
					},
				},
			},
			existingIPs: []*kubeovnv1.IP{}, // Missing IP will cause GetNetInfoForVm to fail
			wantErr:     true,
		},
		{
			name:    "Nil VM object",
			useNil:  true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()
			for _, ip := range tt.existingIPs {
				_, _ = fakeClient.KubeovnV1().IPs().Create(context.Background(), ip, metav1.CreateOptions{})
			}

			GetKubeOvnClient = func() (KubeOvnClient, error) {
				return fakeClient, nil
			}

			var vm *v1.VirtualMachine
			if !tt.useNil {
				vm = &tt.machine
			}

			got, err := GetKubeovnAnnotationsForVM(vm)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetKubeovnAnnotationsForVM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != len(tt.wantAnns) {
					t.Errorf("GetKubeovnAnnotationsForVM() got %d annotations, want %d", len(got), len(tt.wantAnns))
				}
				for k, v := range tt.wantAnns {
					if got[k] != v {
						t.Errorf("GetKubeovnAnnotationsForVM() annotation[%s] = %v, want %v", k, got[k], v)
					}
				}
			}
		})
	}
}
