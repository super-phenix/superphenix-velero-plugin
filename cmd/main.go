package main

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/super-phenix/superphenix-velero-plugin/pkg/plugin"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

func main() {
	framework.NewServer().
		BindFlags(pflag.CommandLine).
		RegisterBackupItemAction("superphenix.net/backup-virtualmachine", vmBackup).
		Serve()
}

func vmBackup(logger logrus.FieldLogger) (interface{}, error) {
	return plugin.NewVMBackupItemAction(logger), nil
}
