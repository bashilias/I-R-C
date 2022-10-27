package main

// Copyright 2022 Ilias Yacoubi (hi@ilias.sh)

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func getCluster() (*kubernetes.Clientset, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// return clientset
	return kubernetes.NewForConfig(config)

}

func getIngress(clientset kubernetes.Clientset) ([]v1.Ingress, error) {

	// get all ingresses
	ingresses, err := clientset.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	// return ingress items
	return ingresses.Items, nil

}

func inspectIngress(i []v1.Ingress) ([]string, []string, []string) {
	// slice for redirect source
	var sr []string
	// slice for redirect target
	var tg []string
	// slice for ingrulehosts
	var irh []string

	for value := range i {
		ingRuleHost := &i[value].Spec.Rules[0].Host

		for j, k := range i[value].ObjectMeta.Annotations {
			if j == "nginx.ingress.kubernetes.io/configuration-snippet" {
				re := regexp.MustCompile(`\brewrite \^(.*?)\s+redirect;`)
				matches := re.FindAllStringSubmatch(k, -1) // Retrieve all matches

				for i, _ := range matches { // Loop over array
					// create slice of full string (source+target)
					s := strings.Split(matches[i][1], " ")

					source := s[0]
					target := s[len(s)-1]

					sr = append(sr, source)
					tg = append(tg, target)
					irh = append(irh, *ingRuleHost)

				}

			}
		}

	}
	return sr, tg, irh

}

func statusChecker(s string) bool {
	// ignore expired certificates
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	_, err := http.Get(s)
	var resp bool

	if err != nil {
		resp = false
	} else {
		resp = true

	}
	return resp

}

func main() {
	clientset, _ := getCluster()
	ing, _ := getIngress(*clientset)

	sr, tg, inghost := inspectIngress(ing)

	i := 0

	for i < len(sr) {

		source := "http://" + inghost[i] + sr[i]
		target := ""
		// check source

		if strings.HasSuffix(source, "$") {
			source = source[:len(source)-len("$")]
		}

		// check target
		if strings.HasPrefix(tg[i], "http") || strings.HasPrefix(tg[i], "www") {
			target = tg[i]

		} else {
			target = "http://" + inghost[i] + "/" + tg[i]

		}
		i++

		if !statusChecker(target) {
			fmt.Printf("🔴 %s \n\t👉 %s\n\n", source, target)

		}

	}

}
