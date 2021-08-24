package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const ReportPeriod = 30 * time.Second

var (
	sugar   *zap.SugaredLogger
	rootCmd *cobra.Command

	image      string
	replica    int32
	kubeconfig string
)

func Init() {
	logger, _ := zap.NewProduction()
	defer logger.Sync() // flushes buffer, if any

	sugar = logger.Sugar()
	Deployment = fmt.Sprintf(DeploymentFormat, time.Now().Format("0102-150405"))

	rootCmd = &cobra.Command{
		Use:   "./stress [-r num] [--kubeconfig path] image",
		Short: "It is used to measure the degree of mirror acceleration of dragonfly2",
		Long: "First, it starts a job which container image is specified by user. Then it " +
			"calculates the image download time for all pods",
		Example: "./stress -n 20 docker.io/library/busybox:latest",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("need exactly one image")
			}
			image = args[0]
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Start Testing Dragonfly2. Replica=%v. Image=%v\n", replica, image)

			ctx := controllerruntime.SetupSignalHandler()
			c, newCtx := NewController(ctx, replica, image)
			defer DeleteDeployment(c.ClientSet)

			go c.InformerFactory.Start(newCtx.Done())
			go func() {
				time.Sleep(ReportPeriod)
				wait.Until(c.report, ReportPeriod, newCtx.Done())
			}()
			go c.Run(newCtx)
			<-newCtx.Done()
		},
	}
	rootCmd.Flags().Int32VarP(&replica, "replica", "r", 20, "(optional)replica number")
	rootCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "", "", "(optional)absolute path to the kubeconfig file")
}

func main() {
	Init()

	rootCmd.Execute()

}

func (c *Controller) report() {
	fmt.Printf("Desired=%v.Now=%v\n", c.Desired, c.cache.Get())
}
