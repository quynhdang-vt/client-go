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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"os"
	yaml2 "sigs.k8s.io/yaml"
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

func getPodObjectFromYaml(yamlBytes []byte) (*core.Pod, error) {
	jsonBytes, err := yaml2.YAMLToJSON(yamlBytes)
	if err != nil {
		return nil, err
	}
	var res core.Pod
	err = json.Unmarshal(jsonBytes, &res)
	return &res, err
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

/*
Make sure to have namespace aiware and also label for node runmode-engine=true
*/

var testPodYaml string = `
apiVersion: v1
kind: Pod
metadata:
  name: hello
  namespace: aiware
  labels:
    qdhello: "true"
spec:
  restartPolicy: Never
  containers:
  - name: hello
    image: busybox:1.28
    command: ['sh', '-c', 'echo "Hello, Kubernetes!" && sleep 30']
  nodeSelector:
    runmode-engine: "true"
`

/*
createPod returns Failed to Create - err=pods is forbidden: User "system:serviceaccount:default:qdtest" ca
nnot create resource "pods" in API group "" in the namespace "aiware"
*/
func createPod(ctx context.Context, iter int, clientset *kubernetes.Clientset) error {
	// https://stackoverflow.com/questions/53101168/deploy-a-kubernetes-pod-using-golang-code
	// build the pod defination we want to deploy
	// pod := getPodObject()
	pod, err := getPodObjectFromYaml([]byte(testPodYaml))
	pod.Name = fmt.Sprintf("%s-%d-%d", pod.Name, iter, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("Failed to getPodObjectFromYaml - err=%w", err)
	}
	// now create the pod in kubernetes cluster using the clientset
	pod, err = clientset.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to Create - err=%w", err)
	}
	fmt.Printf("Pod created successfully..pod: %s\n.", ToJSONString(pod))
	return nil
}

func convertNode(k8Node *core.Node) Node {
	var res Node
	if k8Node != nil {
		res.Name = k8Node.Name
		res.CreationTimestamp = k8Node.CreationTimestamp.Time

	}
	var hostName string
	for _, a := range k8Node.Status.Addresses {
		if a.Type == core.NodeHostName {
			hostName = a.Address
			break
		}
	}
	res.HostName = hostName
	res.Labels = k8Node.Labels

	if v, b := k8Node.Status.Allocatable.Cpu().AsInt64(); b {
		res.AllocatableCPU = v
	}
	if v, b := k8Node.Status.Allocatable.Memory().AsInt64(); b {
		res.AllocatableMemory = v
	}
	return res
}

type Node struct {
	Name              string            `json:"name"`
	Ips               string            `json:"ips"`
	HostName          string            `json:"hostName"`
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
	AllocatableCPU    int64             `json:"allocatableCPU"`
	AllocatableMemory int64             `json:"allocatableMemory"`
}

func main() {
	// creates the in-cluster config
	progContext, progCancel := context.WithCancel(context.Background())
	defer progCancel()

	stopChan := make(chan os.Signal)
	go func() {
		select {
		case <-stopChan:
			progCancel()
		}
	}()

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)

	namespace := os.Getenv("NAMESPACE")
	verboseNode := os.Getenv("VERBOSE_NODE") == "true"
	verbosePod := os.Getenv("VERBOSE_POD") == "true"
	s := os.Getenv("SLEEP_DURATION")
	var sleepDuration = 1 * time.Minute
	if s != "" {
		if t, parseErr := time.ParseDuration(s); parseErr == nil {
			sleepDuration = t
		}
	}

	if err != nil {
		panic(err.Error())
	}

	// get Names pace
	nsList, err := clientset.CoreV1().Namespaces().List(progContext, metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	if len(nsList.Items) == 0 {
		fmt.Printf("?? no namespace?? \n")
	}
	for i, ns := range nsList.Items {
		fmt.Printf("Namespace[%d]=%s\n", i, ns.Name)
	}
	// get nodes based on selector
	iter := 0
	labelSelector := os.Getenv("LABEL_SELECTOR") // "labelName=labelKey",

	// more sample: https://stackoverflow.com/questions/74544526/dynamicsharedinformer-not-able-to-get-missed-delete-events
	/*
		https://blog.dsb.dev/posts/creating-dynamic-informers/
	*/

	// get the complete pods
	allInformers := informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithNamespace("aiware"))
	podInformer := allInformers.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			newPod := newObj.(*core.Pod)
			fmt.Printf("Found UPDATE POD: Name=%s, status=%s, labels=%v\n", newPod.Name, newPod.Status.Phase, newPod.Labels)
			if qdhello, found := newPod.Labels["qdhello"]; found {
				if qdhello == "true" {
					if newPod.Status.Phase == core.PodSucceeded {
						fmt.Printf("%s Found a complete pod -- %s can I delete?\n", time.Now().Format(time.RFC3339), newPod.Name)
						// want to delete
						err = clientset.CoreV1().Pods(namespace).Delete(progContext, newPod.Name, metav1.DeleteOptions{})
						fmt.Printf("%s -- Delete? err=%v\n", newPod.Name, err)
					}
				}
			}
		},
	})
	allInformers.Start(progContext.Done())

	// ok what to do with the delete?

	for {
		iter++
		fmt.Printf("------------\n%s Next Iteration %d\n------------\n",
			time.Now().Format(time.RFC3339), iter)
		createErr := createPod(progContext, iter, clientset)
		fmt.Printf("createPod returns %v\n", createErr)
		// get nodes
		nodes, err := clientset.CoreV1().Nodes().List(progContext, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d nodes in the cluster with label=%s\n", len(nodes.Items), labelSelector)
		if verboseNode {
			for i, node := range nodes.Items {
				fmt.Printf("Node[%d] =  %+s\n", i, ToJSONString(convertNode(&node)))
			}
		}
		fmt.Println("------------")
		// get pods in all the namespaces by omitting namespace
		// Or specify namespace to get pods in particular namespace
		pods, err := clientset.CoreV1().Pods(namespace).List(progContext, metav1.ListOptions{})
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

		_, err = clientset.CoreV1().Pods(namespace).Get(progContext, "hello", metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("Pod hello not found in %s namespace\n", namespace)
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			panic(err.Error())
		} else {
			fmt.Printf("Found hello pod in default namespace\n")
			err = clientset.CoreV1().Pods(namespace).Delete(progContext, "hello", metav1.DeleteOptions{})
			if err != nil {
				panic(err.Error())
			}
		}
		fmt.Printf("Sleeping for %s....\n", sleepDuration.String())
		time.Sleep(sleepDuration)
	}
}
