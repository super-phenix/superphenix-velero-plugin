package plugin

import (
	"context"
	"testing"

	"github.com/kubeovn/kube-ovn/pkg/apis/kubeovn/v1"
	"github.com/kubeovn/kube-ovn/pkg/client/clientset/versioned/fake"
	"github.com/sirupsen/logrus"
	u "github.com/super-phenix/superphenix-velero-plugin/pkg/util"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kvcore "kubevirt.io/api/core/v1"
)

func TestExecute(t *testing.T) {
	// Mock GetKubeOvnClient
	originalGetKubeOvnClient := u.GetKubeOvnClient
	defer func() { u.GetKubeOvnClient = originalGetKubeOvnClient }()

	// Mock isVMIExcludedByLabel
	originalIsVMIExcludedByLabel := isVMIExcludedByLabel
	defer func() { isVMIExcludedByLabel = originalIsVMIExcludedByLabel }()

	logger := logrus.New()
	action := NewVMBackupItemAction(logger)

	tests := []struct {
		name            string
		vm              *kvcore.VirtualMachine
		backup          *velerov1api.Backup
		existingIPs     []*v1.IP
		excluded        bool
		wantAnnotations map[string]string
		wantErr         bool
	}{
		{
			name: "Successful backup - annotations added to VM",
			vm: &kvcore.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: kvcore.VirtualMachineSpec{
					Template: &kvcore.VirtualMachineInstanceTemplateSpec{
						Spec: kvcore.VirtualMachineInstanceSpec{
							Networks: []kvcore.Network{},
						},
					},
				},
			},
			backup: &velerov1api.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-backup",
				},
				Spec: velerov1api.BackupSpec{
					IncludedResources: []string{"*"},
				},
			},
			existingIPs: []*v1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
					Spec: v1.IPSpec{
						V4IPAddress: "10.0.0.1",
						MacAddress:  "00:00:00:00:00:01",
					},
				},
			},
			excluded: false,
			wantAnnotations: map[string]string{
				"ovn.kubernetes.io/ip_address":  "10.0.0.1",
				"ovn.kubernetes.io/mac_address": "00:00:00:00:00:01",
			},
			wantErr: false,
		},
		{
			name: "Successful backup - existing annotations preserved",
			vm: &kvcore.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm-existing",
					Namespace: "test-ns",
				},
				Spec: kvcore.VirtualMachineSpec{
					Template: &kvcore.VirtualMachineInstanceTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"existing.annotation": "preserved",
							},
						},
						Spec: kvcore.VirtualMachineSpec{
							Template: &kvcore.VirtualMachineInstanceTemplateSpec{
								Spec: kvcore.VirtualMachineInstanceSpec{
									Networks: []kvcore.Network{},
								},
							},
						}.Template.Spec, // reusing the structure
					},
				},
			},
			backup: &velerov1api.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-backup",
				},
				Spec: velerov1api.BackupSpec{
					IncludedResources: []string{"*"},
				},
			},
			existingIPs: []*v1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm-existing.test-ns",
					},
					Spec: v1.IPSpec{
						V4IPAddress: "10.0.0.2",
						MacAddress:  "00:00:00:00:00:02",
					},
				},
			},
			excluded: false,
			wantAnnotations: map[string]string{
				"ovn.kubernetes.io/ip_address":  "10.0.0.2",
				"ovn.kubernetes.io/mac_address": "00:00:00:00:00:02",
				"existing.annotation":           "preserved",
			},
			wantErr: false,
		},
		{
			name:    "Nil backup object",
			vm:      &kvcore.VirtualMachine{},
			backup:  nil,
			wantErr: true,
		},
		{
			name: "Safety check fails - excluded by label",
			vm: &kvcore.VirtualMachine{
				Status: kvcore.VirtualMachineStatus{
					PrintableStatus: kvcore.VirtualMachineStatusRunning,
				},
			},
			backup: &velerov1api.Backup{
				Spec: velerov1api.BackupSpec{
					IncludedResources: []string{"virtualmachineinstances"},
				},
			},
			excluded: true,
			wantErr:  true,
		},
		{
			name: "Multiple secondary networks",
			vm: &kvcore.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: kvcore.VirtualMachineSpec{
					Template: &kvcore.VirtualMachineInstanceTemplateSpec{
						Spec: kvcore.VirtualMachineInstanceSpec{
							Networks: []kvcore.Network{
								{
									Name: "secondary1",
									NetworkSource: kvcore.NetworkSource{
										Multus: &kvcore.MultusNetwork{
											NetworkName: "test-ns/nad1",
										},
									},
								},
								{
									Name: "secondary2",
									NetworkSource: kvcore.NetworkSource{
										Multus: &kvcore.MultusNetwork{
											NetworkName: "test-ns/nad2",
										},
									},
								},
							},
						},
					},
				},
			},
			backup: &velerov1api.Backup{
				Spec: velerov1api.BackupSpec{
					IncludedResources: []string{"*"},
				},
			},
			existingIPs: []*v1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.nad1.test-ns",
					},
					Spec: v1.IPSpec{
						V4IPAddress: "10.0.0.11",
						MacAddress:  "00:00:00:00:00:11",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.nad2.test-ns",
					},
					Spec: v1.IPSpec{
						V4IPAddress: "10.0.0.22",
						MacAddress:  "00:00:00:00:00:22",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
					Spec: v1.IPSpec{
						V4IPAddress: "10.0.0.1",
						MacAddress:  "00:00:00:00:00:01",
					},
				},
			},
			wantAnnotations: map[string]string{
				"nad1.test-ns.ovn.kubernetes.io/ip_address":  "10.0.0.11",
				"nad1.test-ns.ovn.kubernetes.io/mac_address": "00:00:00:00:00:11",
				"nad2.test-ns.ovn.kubernetes.io/ip_address":  "10.0.0.22",
				"nad2.test-ns.ovn.kubernetes.io/mac_address": "00:00:00:00:00:22",
				"ovn.kubernetes.io/ip_address":               "10.0.0.1",
				"ovn.kubernetes.io/mac_address":              "00:00:00:00:00:01",
			},
			wantErr: false,
		},
		{
			name: "Default Multus network",
			vm: &kvcore.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: kvcore.VirtualMachineSpec{
					Template: &kvcore.VirtualMachineInstanceTemplateSpec{
						Spec: kvcore.VirtualMachineInstanceSpec{
							Networks: []kvcore.Network{
								{
									Name: "primary",
									NetworkSource: kvcore.NetworkSource{
										Multus: &kvcore.MultusNetwork{
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
			backup: &velerov1api.Backup{
				Spec: velerov1api.BackupSpec{
					IncludedResources: []string{"*"},
				},
			},
			existingIPs: []*v1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.nad-primary.test-ns",
					},
					Spec: v1.IPSpec{
						V4IPAddress: "10.0.0.33",
						MacAddress:  "00:00:00:00:00:33",
					},
				},
			},
			wantAnnotations: map[string]string{
				"nad-primary.test-ns.ovn.kubernetes.io/ip_address":  "10.0.0.33",
				"nad-primary.test-ns.ovn.kubernetes.io/mac_address": "00:00:00:00:00:33",
			},
			wantErr: false,
		},
		{
			name: "Non-explicit default network + a multus secondary",
			vm: &kvcore.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: "test-ns",
				},
				Spec: kvcore.VirtualMachineSpec{
					Template: &kvcore.VirtualMachineInstanceTemplateSpec{
						Spec: kvcore.VirtualMachineInstanceSpec{
							Networks: []kvcore.Network{
								{
									Name: "secondary",
									NetworkSource: kvcore.NetworkSource{
										Multus: &kvcore.MultusNetwork{
											NetworkName: "test-ns/nad-secondary",
										},
									},
								},
							},
						},
					},
				},
			},
			backup: &velerov1api.Backup{
				Spec: velerov1api.BackupSpec{
					IncludedResources: []string{"*"},
				},
			},
			existingIPs: []*v1.IP{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns",
					},
					Spec: v1.IPSpec{
						V4IPAddress: "10.0.0.1",
						MacAddress:  "00:00:00:00:00:01",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-vm.test-ns.nad-secondary.test-ns",
					},
					Spec: v1.IPSpec{
						V4IPAddress: "10.0.0.44",
						MacAddress:  "00:00:00:00:00:44",
					},
				},
			},
			wantAnnotations: map[string]string{
				"ovn.kubernetes.io/ip_address":                        "10.0.0.1",
				"ovn.kubernetes.io/mac_address":                       "00:00:00:00:00:01",
				"nad-secondary.test-ns.ovn.kubernetes.io/ip_address":  "10.0.0.44",
				"nad-secondary.test-ns.ovn.kubernetes.io/mac_address": "00:00:00:00:00:44",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up fake Kube-OVN client
			fakeClient := fake.NewSimpleClientset()
			for _, ip := range tt.existingIPs {
				_, _ = fakeClient.KubeovnV1().IPs().Create(context.Background(), ip, metav1.CreateOptions{})
			}
			u.GetKubeOvnClient = func() (u.KubeOvnClient, error) {
				return fakeClient, nil
			}

			// Mock isVMIExcludedByLabel
			isVMIExcludedByLabel = func(vm *kvcore.VirtualMachine) (bool, error) {
				return tt.excluded, nil
			}

			// Convert VM to Unstructured
			vmUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.vm)
			if err != nil {
				t.Fatalf("failed to convert VM to unstructured: %v", err)
			}

			obj := &unstructured.Unstructured{Object: vmUnstructured}

			got, _, err := action.Execute(obj, tt.backup)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got == nil {
					t.Errorf("Execute() returned nil item")
					return
				}

				// Convert back to VM to check annotations
				gotVM := new(kvcore.VirtualMachine)
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(got.UnstructuredContent(), gotVM)
				if err != nil {
					t.Fatalf("failed to convert returned item back to VM: %v", err)
				}

				annotations := gotVM.Spec.Template.ObjectMeta.Annotations
				for k, v := range tt.wantAnnotations {
					if annotations[k] != v {
						t.Errorf("Execute() expected annotation %s=%s, got %s", k, v, annotations[k])
					}
				}
			}
		})
	}
}
