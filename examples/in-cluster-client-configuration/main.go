/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Note: the example only works with the code within the same release/branch.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	core "k8s.io/api/core/v1"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

// ToJSONString convert an interface to JSON string
func ToJSONString(data interface{}) string {
	if data == nil {
		return "{}"
	}

	_, isError := data.(error)
	if isError {
		return fmt.Sprintf("%+v", data)
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%+v", data)
	}
	return string(b)
}

func getPodObject() *core.Pod {
	return &core.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "demo",
			},
		},
		Spec: core.PodSpec{
			Containers: []core.Container{
				{
					Name:            "busybox",
					Image:           "busybox",
					ImagePullPolicy: core.PullIfNotPresent,
					Command: []string{
						"sleep",
						"3600",
					},
				},
			},
		},
	}
}
func createPod(ctx context.Context, clientset *kubernetes.Clientset) {
	// https://stackoverflow.com/questions/53101168/deploy-a-kubernetes-pod-using-golang-code
	// build the pod defination we want to deploy
	pod := getPodObject()

	var err error
	// now create the pod in kubernetes cluster using the clientset
	pod, err = clientset.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Pod created successfully..pod: %s\n.", pod.Name)
}
func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	createPod(context.TODO(), clientset)
	namespace := os.Getenv("NAMESPACE")
	verboseNode := os.Getenv("VERBOSE_NODE") == "true"
	verbosePod := os.Getenv("VERBOSE_POD") == "true"
	s := os.Getenv("SLEEP_DURATION")
	var sleepDuration = 20 * time.Second
	if s != "" {
		if t, parseErr := time.ParseDuration(s); parseErr == nil {
			sleepDuration = t
		}
	}

	if err != nil {
		panic(err.Error())
	}
	iter := 0
	for {
		iter++
		// get nodes
		fmt.Printf("------------\n%s Next Iteration %d\n------------\n",
			time.Now().Format(time.RFC3339), iter)
		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d nodes in the cluster\n", len(nodes.Items))
		if verboseNode {
			for i, node := range nodes.Items {
				fmt.Printf("Node[%d] =  %+s\n", i, ToJSONString(node))
			}
		}
		fmt.Println("------------")
		// get pods in all the namespaces by omitting namespace
		// Or specify namespace to get pods in particular namespace
		pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

		if verbosePod {
			for i, pod := range pods.Items {
				fmt.Printf("POD [%d] %+v\n", i, pod)
			}
		}
		fmt.Println("------------")
		// Examples for error handling:
		// - Use helper functions e.g. errors.IsNotFound()
		// - And/or cast to StatusError and use its properties like e.g. ErrStatus.Message
		_, err = clientset.CoreV1().Pods("default").Get(context.TODO(), "example-xxxxx", metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("Pod example-xxxxx not found in default namespace\n")
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Found example-xxxxx pod in default namespace\n")
		}
		fmt.Printf("Sleeping for %s....\n", sleepDuration.String())
		time.Sleep(sleepDuration)
	}
}
