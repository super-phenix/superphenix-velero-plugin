package plugin

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

type VMBackupItemAction struct {
	log logrus.FieldLogger
}

func NewVMBackupItemAction(logger logrus.FieldLogger) *VMBackupItemAction {
	return &VMBackupItemAction{
		log: logger,
	}
}

func (v *VMBackupItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"virtualmachines.kubevirt.io"},
	}, nil
}

func (v *VMBackupItemAction) Execute(item runtime.Unstructured, backup *velerov1api.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	v.log.Info("Executing VMBackupItemAction")

	// No backup, errors out
	if backup == nil {
		return nil, nil, fmt.Errorf("backup object is nil")
	}

	// Retrieve the VM we are trying to backup
	vm := new(kvcore.VirtualMachine)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), vm); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	// Check if we can safely backup the VM
	safe, err := v.canBeSafelyBackedUp(vm, backup)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	if !safe {
		return nil, nil, fmt.Errorf("VM cannot be safely backed up")
	}

	// We can skip all checks that ensure consistency if we just want to backup for metadata purposes
	if !util.IsMetadataBackup(backup) {
		skipVolume := func(volume kvcore.Volume) bool {
			return volumeInDVTemplates(volume, vm)
		}

		restore, err := util.RestorePossible(vm.Spec.Template.Spec.Volumes, backup, vm.Namespace, skipVolume, v.log)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		if !restore {
			return nil, nil, fmt.Errorf("VM would not be restored correctly")
		}
	}

	vmUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: vmUnstructured}, nil, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Functions imported from https://github.com/kubevirt/kubevirt-velero-plugin/blob/main/pkg/plugin/vm_backup_item_action.go
// Those functions aren't public, but we need to only backup VMs if the Kubevirt Velero plugin thinks we should/
// We apply the same exact inclusion/exclusion logic.
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (p *VMBackupItemAction) canBeSafelyBackedUp(vm *kvcore.VirtualMachine, backup *velerov1api.Backup) (bool, error) {
	isRunning := vm.Status.PrintableStatus == kvcore.VirtualMachineStatusStarting || vm.Status.PrintableStatus == kvcore.VirtualMachineStatusRunning
	if !isRunning {
		return true, nil
	}

	if !util.IsResourceInBackup("virtualmachineinstances", backup) {
		p.log.Info("Backup of a running VM does not contain VMI")
		return false, nil
	}

	excluded, err := isVMIExcludedByLabel(vm)
	if err != nil {
		return false, errors.WithStack(err)
	}

	if excluded {
		p.log.Info("VM is running but VMI is not included in the backup")
		return false, nil
	}

	if !util.IsResourceInBackup("pods", backup) && util.IsResourceInBackup("persistentvolumeclaims", backup) {
		p.log.Info("Backup of a running VM does not contain Pod but contains PVC")
		return false, nil
	}

	return true, nil
}

// This is assigned to a variable so it can be replaced by a mock function in tests
var isVMIExcludedByLabel = func(vm *kvcore.VirtualMachine) (bool, error) {
	client, err := util.GetKubeVirtclient()
	if err != nil {
		return false, err
	}

	vmi, err := (*client).VirtualMachineInstance(vm.Namespace).Get(context.Background(), vm.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	labels := vmi.GetLabels()
	if labels == nil {
		return false, nil
	}

	label, ok := labels[util.VeleroExcludeLabel]
	return ok && label == "true", nil
}

func volumeInDVTemplates(volume kvcore.Volume, vm *kvcore.VirtualMachine) bool {
	for _, template := range vm.Spec.DataVolumeTemplates {
		if template.Name == volume.VolumeSource.DataVolume.Name {
			return true
		}
	}

	return false
}
