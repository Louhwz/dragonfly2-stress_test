package main

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	Deployment       string
	DeploymentFormat = "dragonfly-stress-test-%v"
)

func CreateDeployment(imageName string, replica int32, clientset kubernetes.Interface) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Deployment,
			Namespace: EventNameSpace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: Int32Ptr(replica),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "dragonfly-stress-test",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "dragonfly-stress-test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "stress-test",
							Image:           imageName,
							ImagePullPolicy: "Always",
							Command:         []string{"sleep"},
							Args:            []string{"36000"},
						},
					},
				},
			},
		},
	}
	_, err := clientset.AppsV1().Deployments(EventNameSpace).Create(context.TODO(), deployment, metav1.CreateOptions{})
	return err
}

func DeleteDeployment(clientset kubernetes.Interface) {
	defer func() {
		recover()
	}()
	_ = clientset.AppsV1().Deployments(EventNameSpace).Delete(context.TODO(), Deployment, metav1.DeleteOptions{})
}

func Int32Ptr(i int32) *int32 {
	return &i
}
