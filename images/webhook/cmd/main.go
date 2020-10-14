// Copyright 2020 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Binary main runsc serves a mutating Kubernetes webhook.
package main

import (
	"flag"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gvisor.dev/images/webhook/pkg/injector"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8snet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	address   = flag.String("address", "", "The ip address the admission webhook serves on. If unspecified, a public address is selected automatically.")
	port      = flag.Int("port", 0, "The port the admission webhook serves on.")
	podLabels = flag.String("pod-namespace-labels", "", "A comma-separated namespace label selector, the admission webhook will only take effect on pods in selected namespaces, e.g. `label1,label2`.")
	logLevel  = flag.String("log-level", "info", "Set admission webhook log level. Available options: debug, info, warn, error, fatal, panic.")
)

func main() {
	flag.Parse()

	lvl, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logrus.WithError(err).Fatalf("Failed to parse log level %q", *logLevel)
	}
	logrus.SetLevel(lvl)

	if err := run(); err != nil {
		logrus.Fatal(err)
	}
}

func run() error {
	logrus.Infof("Starting %s\n", injector.Name)

	// Create client config.
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return errors.Wrap(err, "create in cluster config")
	}

	// Create clientset.
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "create kubernetes client")
	}

	if err := injector.CreateConfiguration(clientset, parsePodLabels()); err != nil {
		return errors.Wrap(err, "create webhook configuration")
	}

	if err := startWebhookHTTPS(clientset); err != nil {
		return errors.Wrap(err, "start webhook https server")
	}

	return nil
}

func parsePodLabels() *metav1.LabelSelector {
	rv := &metav1.LabelSelector{}
	for _, s := range strings.Split(*podLabels, ",") {
		req := metav1.LabelSelectorRequirement{
			Key:      strings.TrimSpace(s),
			Operator: "Exists",
		}
		rv.MatchExpressions = append(rv.MatchExpressions, req)
	}
	return rv
}

func startWebhookHTTPS(clientset kubernetes.Interface) error {
	logrus.Info("Starting HTTPS handler")
	defer logrus.Info("Stopping HTTPS handler")

	if *address == "" {
		ip, err := k8snet.ChooseHostInterface()
		if err != nil {
			return errors.Wrap(err, "select ip address")
		}
		*address = ip.String()
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			injector.Admit(w, r)
		}))
	server := &http.Server{
		// Listen on all addresses.
		Addr:      net.JoinHostPort(*address, strconv.Itoa(*port)),
		TLSConfig: injector.GetTLSConfig(),
		Handler:   mux,
	}
	if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		return errors.Wrap(err, "start HTTPS handler")
	}
	return nil
}
