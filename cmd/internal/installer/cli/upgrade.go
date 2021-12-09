package cli

import (
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/installer/internal/duration"
	"github.com/epinio/installer/internal/installer"
	"github.com/epinio/installer/internal/kubernetes"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var CmdUpgrade = &cobra.Command{
	Use:   "upgrade",
	Short: "upgrade Epinio in your configured kubernetes cluster",
	Long:  `upgrade Epinio PaaS in your configured kubernetes cluster`,
	Args:  cobra.ExactArgs(0),
	RunE:  upgrade,
}

func upgrade(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	exitfIfError(checkDependencies(), "Cannot operate")

	ctx := cmd.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return err
	}

	log := tracelog.NewLogger().WithName("EpinioUpgrader")

	path := viper.GetString("manifest")
	m, err := installer.Load(path)
	if err != nil {
		return err
	}

	p, err := installer.BuildPlan(m.Components)
	if err != nil {
		return err
	}

	log.Info("plan", "components", p.String())

	ca := installer.NewComponentActions(cluster, log, duration.ToDeployment())
	act := installer.NewUpgrade(cluster, log, ca)

	err = installer.WalkSerially(ctx, p, act)
	if err != nil {
		return err
	}

	return nil
}
