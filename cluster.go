package main

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func ConnectToCluster() *kubernetes.Clientset {
	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		sugar.Error("Can't find kubeconfig. Please specify kubeconfg.")
		os.Exit(1)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		sugar.Error("Can't create kube clientset. Please specify kubeconfg.")
		os.Exit(1)
	}
	return clientset
}
