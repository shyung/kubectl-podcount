package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type PodInfo struct {
	pod_total_count                int
	pod_pending_count              int
	pod_running_count              int
	pod_ready_count                int
	pod_running_not_ready_count    int
	pod_unscheduled_count          int
	pod_uninitialized_count        int
	pod_containers_not_ready_count int
}

func (pi *PodInfo) output() {
	fmt.Printf("pod_total_count:%d\n", pi.pod_total_count)
	fmt.Printf("pod_pending_count:%d\n", pi.pod_pending_count)
	fmt.Printf("pod_running_count:%d\n", pi.pod_running_count)
	fmt.Printf("pod_ready_count:%d\n", pi.pod_ready_count)
	fmt.Printf("pod_running_not_ready_count:%d\n", pi.pod_running_not_ready_count)
	fmt.Printf("pod_unscheduled_count:%d\n", pi.pod_unscheduled_count)
	fmt.Printf("pod_uninitialized_count:%d\n", pi.pod_uninitialized_count)
	fmt.Printf("pod_containers_not_ready_count:%d\n", pi.pod_containers_not_ready_count)
}

func main() {
	namespace := flag.String("n", "", "namespace to summary, default all namespaces")
	var kubecfg *string
	if home := homedir.HomeDir(); home != "" {
		kubecfg = flag.String("kubecfg", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubecfg = flag.String("kubecfg", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	restConfig, err := clientcmd.BuildConfigFromFlags("", *kubecfg)
	if err != nil {
		fmt.Printf("build config from flags failed, err: %v\n", err)
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		fmt.Printf("new kubeclient for config failed, err: %v\n", err)
		os.Exit(1)
	}

	namespaces := []string{}
	if *namespace == "" {
		namespaceList, err := kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("list namespaces failed, err: %v\n", err)
			os.Exit(1)
		}

		for i := range namespaceList.Items {
			namespaces = append(namespaces, namespaceList.Items[i].Name)
		}
	} else {
		ns, err := kubeClient.CoreV1().Namespaces().Get(context.TODO(), *namespace, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("get namespace %s failed, err: %v\n", *namespace, err)
			os.Exit(1)
		}

		namespaces = append(namespaces, ns.Name)
	}

	podInfo := &PodInfo{}
	defer podInfo.output()
	for i := range namespaces {
		ns := namespaces[i]
		pods, err := kubeClient.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			fmt.Printf("list pods in namespace %s failed, err: %v\n", ns, err)
			continue
		}
		podInfo.pod_total_count += len(pods.Items)
		for i := range pods.Items {
			pod := &pods.Items[i]

			var (
				podReady        bool
				containersReady bool
				podInitialized  bool
				podScheduled    bool
			)

			for j := range pod.Status.Conditions {
				condition := &pod.Status.Conditions[j]
				switch condition.Type {
				case corev1.PodReady:
					podReady = condition.Status == corev1.ConditionTrue
				case corev1.ContainersReady:
					containersReady = condition.Status == corev1.ConditionTrue
				case corev1.PodInitialized:
					podInitialized = condition.Status == corev1.ConditionTrue
				case corev1.PodScheduled:
					podScheduled = condition.Status == corev1.ConditionTrue
				}
			}

			if podReady {
				podInfo.pod_ready_count += 1
				podInfo.pod_running_count += 1
				continue
			}

			switch pod.Status.Phase {
			case corev1.PodRunning:
				podInfo.pod_running_count += 1
				podInfo.pod_running_not_ready_count += 1
			case corev1.PodPending:
				podInfo.pod_pending_count += 1
			}

			if !podScheduled {
				podInfo.pod_unscheduled_count += 1
				continue
			}

			if !podInitialized {
				podInfo.pod_uninitialized_count += 1
				continue
			}

			if !containersReady {
				podInfo.pod_containers_not_ready_count += 1
			}
		}
	}
}
